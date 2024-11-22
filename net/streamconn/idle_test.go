// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package streamconn_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"cloudeng.io/errors"
	"github.com/cosnicolaou/automation/net/streamconn"
)

func TestIdleTime(t *testing.T) {
	timer := streamconn.NewIdleTimer(10 * time.Minute)
	if got, want := timer.Expired(), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	time.Sleep(time.Second)
	n1 := timer.Remaining()
	if got, want := timer.Remaining(), 10*time.Minute; got >= want {
		t.Errorf("got %v, want > %v", timer.Remaining(), 10*time.Minute)
	}
	timer.Reset()
	n2 := timer.Remaining()
	if n1 >= n2 {
		t.Errorf("remaining time n2 should be less than n1 %v < %v", n1, n2)
	}

	timer = streamconn.NewIdleTimer(10 * time.Millisecond)
	time.Sleep(time.Second)
	if got, want := timer.Expired(), true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWatcherStartStop(t *testing.T) {
	ctx := context.Background()
	timer := streamconn.NewIdleTimer(time.Millisecond)

	readyCh := make(chan struct{})
	waitCh := make(chan time.Time)

	mytime := time.Now()
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		timer.Wait(ctx, func(ctx context.Context) error {
			<-readyCh
			waitCh <- mytime
			close(waitCh)
			return nil
		})
		wg.Done()
	}()

	// Allow thje expire function to be execute and capture its
	// output (via waitCh).
	readyCh <- struct{}{}
	if got, want := <-waitCh, mytime; !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
	fmt.Printf("wainting for stop\n")
	if err := timer.StopWait(ctx); err != nil {
		t.Fatal(err)
	}
	fmt.Printf("wg.Wait\n")
	wg.Wait()
	t.FailNow()

	timer = streamconn.NewIdleTimer(time.Hour)
	wg.Add(1)
	go func() {
		timer.Wait(ctx, func(ctx context.Context) error {
			<-readyCh
			return nil
		})
		wg.Done()
	}()

	var errs errors.M
	go func() {
		//errs.Append(timer.StopWait(ctx))
	}()
	wg.Wait()

	if errs.Err() != nil {
		t.Fatal(errs.Err())
	}

	fmt.Printf("er %v\n", errs.Err())
	t.Fail()
	/*
		ctx, cancel := context.WithCancel(context.Background())


		var errs errors.M
		go timer.Watcher(ctx, func(ctx context.Context) error {
			cancel()
			errs.Append(errors.New("error"))
			return errs.Err()
		})




		if err := timer.StopWatcher(ctx); err != nil {
			t.Fatal(err)
		}
	*/
}
