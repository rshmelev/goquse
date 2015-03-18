package goquse

// type Sleeper interface {
// 	// may be called from one thread
// 	ShouldTryAgainBeforeSleeping() bool
// 	GotSomethingToDo()
// 	Sleep()
// 	// can be called frequently by another thread
// 	WakeUpIfSleeping() bool
// }

// type Sleepable struct {
// 	waking          chan struct{}
// 	sleepnotify     chan struct{}
// 	createdProperly bool
// 	preparedToSleep bool
// }

// func (s *Sleepable) ShouldTryAgainBeforeSleeping() bool {
// 	s.ensureImOk()
// 	if !preparedToSleep {
// 		preparedToSleep = true
// 		return false
// 	}
// 	return true
// }
// func (s *Sleepable) GotSomethingToDo() {
// 	s.ensureImOk()
// 	preparedToSleep = false
// }

// func (s *Sleepable) SleepOrTryAgainLater() {
// 	s.ensureImOk()
// 	s.sleepnotify <- ball
// 	a, ok := <-s.sh
// 	preparedToSleep = false
// }

// func (s *Sleepable) WakeUpIfSleeping() bool {
// 	select {
// 	case <-s.sleepnotify:
// 		s.waking <- ball
// 		return true
// 	default:
// 		return false
// 	}
// }

// func MakeSleeper() *Sleeper {
// 	return &Sleepable{
// 		waking:          make(chan struct{}, 2),
// 		sleepnotify:     make(chan struct{}, 2),
// 		createdProperly: true,
// 	}
// }

// func (s *Sleepable) ensureImOk() {
// 	if !s.createdProperly {
// 		panic("Sleepable not created properly")
// 	}
// }
