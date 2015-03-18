package goquse

import (
	"encoding/json"
	"time"
)

//=============================================================================

func MakeQueueItem(QueueId string, Task interface{}, MaxExecutionTime time.Duration, ResultHandler QueueResultHandlerFunc) *QueueItem {
	i := &QueueItem{
		// incoming
		QueueId:          QueueId,
		Task:             Task,
		MaxExecutionTime: MaxExecutionTime,

		// results
		Result:        nil,
		Error:         nil,
		Timings:       make(map[int8]time.Time),
		ResultHandler: ResultHandler,

		cancelChannel: make(chan struct{}),
		finishChannel: make(chan struct{}),
	}
	i.SetStatus(QueueItemStatusCreated)
	return i
}

type QueueItem struct {
	QueueId          string
	Task             interface{}
	MaxExecutionTime time.Duration

	ItemStatus    int8
	Result        interface{}
	Error         error
	Timings       map[int8]time.Time
	ResultHandler QueueResultHandlerFunc

	// will be set by user if he wants to track all process states
	NotificationsChannel chan int

	// used to pass cancel to executor
	cancelChannel chan struct{}
	cancelled     bool
	// signals that item execution is over, e.g. timeout or (un)successful execution
	finishChannel chan struct{}
}

// should be done from main queue thread only
func (qi *QueueItem) SetStatus(status int8) {
	if status != 0 {
		//println(QueueItemStatuses[status] + " " + toString(qi.Task))
	}
	qi.ItemStatus = status
	qi.Timings[status] = time.Now().UTC()
	if qi.NotificationsChannel != nil {
		select {
		case qi.NotificationsChannel <- int(status):
		default:
		}
	}
}

// for internal use, User can cancel task by queue.Cancel(task)
func (qi *QueueItem) cancel() {
	if qi.cancelled {
		return
	}
	qi.cancelled = true
	qi.SetStatus(QueueItemStatusCancelled)
	close(qi.cancelChannel)
}
func (qi *QueueItem) SetResult(r interface{}) {
	if qi.cancelled || qi.ItemStatus == QueueItemStatusCancelled || qi.ItemStatus == QueueItemStatusFinished {
		return
	}
	qi.Result = r
}
func (qi *QueueItem) GetError() error {
	return qi.Error
}
func (qi *QueueItem) IsTimedOut() bool {
	return qi.Error == TimeoutError
}
func (qi *QueueItem) IsCancelled() bool {
	return qi.Error == CancelledError
}
func (qi *QueueItem) IsCancelledOrTimedOut() bool {
	return qi.IsCancelled() || qi.IsTimedOut()
}

func (qi *QueueItem) Wait(forgetChannel chan struct{}) error {
	select {
	case <-forgetChannel:
		qi.cancel()
		return SkippedError
	case <-qi.finishChannel:
		return qi.Error
	}
}

// interface implementations for executor
func (qi *QueueItem) GetResult() interface{} {
	return qi.Result
}
func (qi *QueueItem) GetQueueId() string {
	return qi.QueueId
}
func (qi *QueueItem) GetTask() interface{} {
	return qi.Task
}
func (qi *QueueItem) GetCancelChannel() chan struct{} {
	return qi.cancelChannel
}
func (qi *QueueItem) GetMaxExecutionTime() time.Duration {
	return qi.MaxExecutionTime
}

//===========================

func (qi *QueueItem) SetError(e error) {
	qi.Error = e
}
func (qi *QueueItem) GetFinishChannel() chan struct{} {
	return qi.finishChannel
}
func (qi *QueueItem) GetResultHandler() QueueResultHandlerFunc {
	return qi.ResultHandler
}

//----------------

type ExecutionResult struct {
	item BasicQueueItem
	err  error
}

type queueRequest struct {
	queueId string
	ch      chan int
	action  int8
}

type Queue struct {
	Name          string
	Executor      QueueExecutorFunc
	ResultHandler QueueResultHandlerFunc

	pending     chan BasicQueueItem
	currentTask BasicQueueItem

	finished chan *ExecutionResult
	incoming chan BasicQueueItem
	cancel   chan BasicQueueItem
	requests chan *queueRequest

	cleanup    []*queueRequest
	alive      bool
	inprogress int
}

func (q *Queue) Manage() {
	q.alive = true
	for {
		select {
		case t := <-q.finished:
			t.item.SetError(t.err)
			t.item.SetStatus(QueueItemStatusFinished)

			q.currentTask = nil
			close(t.item.GetFinishChannel())
			q.inprogress--

			if q.ResultHandler != nil {
				go q.ResultHandler(t.item)
			}
			if t.item.GetResultHandler() != nil {
				go t.item.GetResultHandler()(t.item)
			}
			if q.inprogress == 0 && len(q.cleanup) > 0 {
				// empty!
				for _, c := range q.cleanup {
					close(c.ch)
				}
				q.cleanup = []*queueRequest{}
			}
			if q.inprogress != 0 {
				q.shiftTheQueue()
			}
		case t := <-q.incoming:
			t.SetStatus(QueueItemStatusPending)
			q.pending <- t
			q.inprogress++
			if q.inprogress == 1 {
				q.shiftTheQueue()
			}
		case t := <-q.cancel:
			t.cancel()
		case t := <-q.requests:
			if t.action == QueueActionShutdown {
				q.alive = false
			}
			if t.action == QueueActionCleanup || t.action == QueueActionShutdown {
				if !q.alive || q.inprogress == 0 {
					close(t.ch)
					continue
				}
				q.cleanup = append(q.cleanup, t)
				if q.currentTask != nil {
					q.currentTask.cancel()
				}
			}
		}
	}
}

// shift pending queue, mark task as running... and go go go until q.finished is filled
func (q *Queue) shiftTheQueue() {
	p := <-q.pending
	p.SetStatus(QueueItemStatusRunning)
	q.currentTask = p

	switch {
	case len(q.cleanup) == 0:
		go q.executeTaskOrTimeout(p)
	default:
		q.finished <- &ExecutionResult{item: p, err: SkippedError}
	}

}

func CreateQueue(name string, executor QueueExecutorFunc, reshandler QueueResultHandlerFunc) IQueue {
	q := &Queue{
		Name:          name,
		Executor:      executor,
		ResultHandler: reshandler,
		pending:       make(chan BasicQueueItem, 1000),

		finished: make(chan *ExecutionResult, 10),
		incoming: make(chan BasicQueueItem, 10),
		cancel:   make(chan BasicQueueItem, 0),
		requests: make(chan *queueRequest, 10),

		cleanup: []*queueRequest{},
	}
	return q
}

func (q *Queue) PutAsync(item BasicQueueItem) {
	q.incoming <- item
}

// nobody should use it, i guess... idea of queue is asyncness
func (q *Queue) PutSync(item BasicQueueItem, forgetItChannel ...chan struct{}) error {
	q.PutAsync(item)
	var f chan struct{}
	if len(forgetItChannel) > 0 {
		f = forgetItChannel[0]
	}
	err := item.Wait(f)
	return err
}

func (q *Queue) SetExecutor(e QueueExecutorFunc) {
	q.Executor = e
}

func (q *Queue) executeTaskOrTimeout(t BasicQueueItem) error {
	ch := make(chan error, 2)

	var err error
	select {
	case <-(t.GetCancelChannel()):
		q.finished <- &ExecutionResult{item: t, err: SkippedError}
		return SkippedError
	default:
		go func() {
			err := q.Executor(t)
			ch <- err
		}()
	}

	select {
	case <-time.After(t.GetMaxExecutionTime() + time.Millisecond*50):
		err = TimeoutError
		t.cancel()
	case <-t.GetCancelChannel():
		err = SkippedError
	case e := <-ch:
		err = e
	}

	res := &ExecutionResult{item: t, err: err}
	q.finished <- res

	return err
}

func (q *Queue) Request(action int) {
	if !q.alive {
		return
	}
	ch := make(chan int)
	r := &queueRequest{
		ch:     ch,
		action: int8(action),
	}
	q.requests <- r
	<-ch
}
func (q *Queue) Cancel(t QueueTaskWrapper) {
	i, _ := t.(BasicQueueItem)
	q.cancel <- i
}
func (q *Queue) GetName() string {
	return q.Name
}
func (q *Queue) Cleanup() {
	q.Request(QueueActionCleanup)
}
func (q *Queue) Shutdown() {
	q.Request(QueueActionShutdown)
}

func (q *Queue) MakeQueueItem(Task interface{}, MaxExecutionTime time.Duration, ResultHandler QueueResultHandlerFunc) BasicQueueItem {
	return MakeQueueItem(q.Name, Task, MaxExecutionTime, ResultHandler)
}

// debug
func toString(o interface{}) string {
	b, _ := json.Marshal(o)
	return string(b)
}
