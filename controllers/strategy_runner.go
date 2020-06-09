package controllers

import (
	"time"
	batonv1 "trsnium.com/baton/api/v1"
)

type BatonStrategiesyRunner struct {
	baton    batonv1.Baton
	stopFlag bool
}

func (r *BatonStrategiesyRunner) Run() error {
	r.stopFlag = make(chan bool)
	go func() {
		for {
			select {
			case <-time.After(r.baton.Spec.IntervalSec * time.Second):
			case <-r.stopFlag:
				return
			}
		}
	}()
}

func (r *BatonStrategiesyRunner) Stop() {
	r.stopFlag <- true
}

func (r *BatonStrategiesyRunner) IsUpdatedBatonStrategies(baton batonv1.Baton) bool {
}

func NewBatonStrategiesyRunner(baton batonv1.Baton) BatonStrategiesyRunner {
	return BatonStrategiesyRunner{
		baton: baton,
	}
}
