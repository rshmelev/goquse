package goquse

import (
	"errors"
	"time"
)

type QueueCreatorFunc func(qname string, executor QueueExecutorFunc, reshandler QueueResultHandlerFunc) IQueue

type BasicQueueItem interface {
	QueueExecutionResult
	SetError(e error)
	SetStatus(status int8)
	GetFinishChannel() chan struct{}
	GetResultHandler() QueueResultHandlerFunc
	cancel()
	Wait(forgetChannel chan struct{}) error
}

type QueueTaskWrapper interface {
	GetQueueId() string
	GetTask() interface{}
	GetCancelChannel() chan struct{}
	GetMaxExecutionTime() time.Duration
	SetResult(r interface{})
	IsCancelled() bool
	IsTimedOut() bool
	IsCancelledOrTimedOut() bool
}

type QueueExecutionResult interface {
	QueueTaskWrapper
	GetResult() interface{}
	GetError() error
}

type QueueExecutorFunc func(item QueueTaskWrapper) error
type QueueResultHandlerFunc func(item QueueExecutionResult)

type IBasicQueue interface {
	PutAsync(item BasicQueueItem)
	PutSync(item BasicQueueItem, forgetItChan ...chan struct{}) error
	SetExecutor(QueueExecutorFunc)
	Cancel(item QueueTaskWrapper)
	Shutdown()
	Cleanup()
	Manage()
}
type IQueue interface {
	IBasicQueue
	// SetResultHandler(QueueResultHandlerFunc)
	Request(action int)
	GetName() string
	MakeQueueItem(Task interface{}, MaxExecutionTime time.Duration, ResultHandler QueueResultHandlerFunc) BasicQueueItem
}

const (
	QueueItemStatusCreated   = iota
	QueueItemStatusPending   = iota
	QueueItemStatusRunning   = iota
	QueueItemStatusCancelled = iota
	QueueItemStatusFinished  = iota
)

var QueueItemStatuses []string = []string{"created", "pending", "running", "cancelled", "finished"}

const (
	QueueActionCleanup  = iota
	QueueActionShutdown = iota
)

var TimeoutError error = errors.New("queued item timeout")
var CancelledError error = errors.New("queued item execution cancelled")
var SkippedError error = errors.New("queued item is not interesting anymore")

var ball struct{} = struct{}{}
