package goquse

import "time"

var allQueuesSyn string = "*_all_queues_*"

type IQueues interface {
	IBasicQueue
	/*
		PutAsync(item BasicQueueItem)
		PutSync(item BasicQueueItem, forgetItChan ...chan struct{}) error
		SetExecutor(QueueExecutorFunc)
		Cancel(item QueueTaskWrapper)
		Shutdown()
		Manage()
	*/
	Request(queueId string, action int)
	MakeQueueItem(queueId string, Task interface{}, MaxExecutionTime time.Duration, ResultHandler QueueResultHandlerFunc) BasicQueueItem

	SetQueueCreator(qc QueueCreatorFunc)
	ShutdownQueue(queueId string)
	CleanupQueue(queueId string)
}

func CreateQueues(executor QueueExecutorFunc, reshandler QueueResultHandlerFunc, qcreator QueueCreatorFunc) IQueues {
	return &Queues{
		queues: make(map[string]IQueue),

		incoming: make(chan BasicQueueItem, 1000),
		cancel:   make(chan BasicQueueItem, 10),
		requests: make(chan *queueRequest, 10),

		Executor:      executor,
		ResultHandler: reshandler,
		QueueCreator:  qcreator,
	}
}

type Queues struct {
	queues map[string]IQueue

	incoming chan BasicQueueItem
	cancel   chan BasicQueueItem
	requests chan *queueRequest

	Executor      QueueExecutorFunc
	ResultHandler QueueResultHandlerFunc
	QueueCreator  QueueCreatorFunc
}

func (qs *Queues) Manage() {
	for {
		select {
		case t := <-qs.incoming:
			if t == nil {
				continue
			}
			qid := t.GetQueueId()
			q, ok := qs.queues[qid]
			if !ok {
				if qs.QueueCreator != nil {
					q = qs.QueueCreator(qid, qs.Executor, qs.ResultHandler)
				} else {
					q = CreateQueue(qid, qs.Executor, qs.ResultHandler)
				}
				go q.Manage()
				qs.queues[qid] = q
			}
			q.PutAsync(t)
		case t := <-qs.cancel:
			if t == nil {
				continue
			}
			qid := t.GetQueueId()
			if q, ok := qs.queues[qid]; ok {
				q.Cancel(t)
			}
		case r := <-qs.requests:
			if r.queueId == allQueuesSyn {
				for _, q := range qs.queues {
					q.Request(int(r.action))
				}
				close(r.ch)
				if r.action == QueueActionShutdown {
					delete(qs.queues, r.queueId)
				}
			} else {
				if q, ok := qs.queues[r.queueId]; ok {
					q.Request(int(r.action))
				}
				close(r.ch)
				if r.action == QueueActionShutdown {
					break
				}
			}

		}
	}
}

func (qs *Queues) PutAsync(item BasicQueueItem) {
	qs.incoming <- item
}
func (qs *Queues) PutSync(item BasicQueueItem, forgetItChan ...chan struct{}) error {
	qs.PutAsync(item)
	var f chan struct{}
	if len(forgetItChan) > 0 {
		f = forgetItChan[0]
	}
	err := item.Wait(f)
	return err
}
func (qs *Queues) SetExecutor(f QueueExecutorFunc) {
	qs.Executor = f
}
func (qs *Queues) SetQueueCreator(f QueueCreatorFunc) {
	qs.QueueCreator = f
}
func (qs *Queues) Cancel(item QueueTaskWrapper) {
	i, _ := item.(BasicQueueItem)
	qs.cancel <- i
}
func (qs *Queues) Request(queueId string, action int) {
	ch := make(chan int)
	r := &queueRequest{
		queueId: queueId,
		ch:      ch,
		action:  int8(action),
	}
	qs.requests <- r
	<-ch
}
func (qs *Queues) Cleanup() {
	qs.Request(allQueuesSyn, QueueActionCleanup)
}
func (qs *Queues) Shutdown() {
	qs.Request(allQueuesSyn, QueueActionShutdown)
}
func (qs *Queues) ShutdownQueue(q string) {
	qs.Request(q, QueueActionShutdown)
}
func (qs *Queues) CleanupQueue(q string) {
	qs.Request(q, QueueActionCleanup)
}
func (qs *Queues) MakeQueueItem(queueId string, Task interface{}, MaxExecutionTime time.Duration, ResultHandler QueueResultHandlerFunc) BasicQueueItem {
	return MakeQueueItem(queueId, Task, MaxExecutionTime, ResultHandler)
}
