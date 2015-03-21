// Copyright 2014 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License. See the AUTHORS file
// for names of contributors.
//
// Author: Spencer Kimball (spencer.kimball@gmail.com)

package storage

import (
	"sort"
	"testing"

	"github.com/cockroachdb/cockroach/storage/engine"
)

// TestIDAllocator creates an ID allocator which allocates from
// the Raft ID generator system key in blocks of 10 with a minimum
// ID value of 2 and then starts up 10 goroutines each allocating
// 10 IDs. All goroutines deposit the allocated IDs into a final
// channel, which is queried at the end to ensure that all IDs
// from 2 to 101 are present.
func TestIDAllocator(t *testing.T) {
	store, _ := createTestStore(t)
	allocd := make(chan int, 100)
	idAlloc, err := NewIDAllocator(engine.KeyRaftIDGenerator, store.db, 2, 10)
	if err != nil {
		t.Errorf("failed to create IDAllocator: %v", err)
	}

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				id, _ := idAlloc.Allocate()
				allocd <- int(id)
			}
		}()
	}

	// Verify all IDs accounted for.
	ids := make([]int, 100)
	for i := 0; i < 100; i++ {
		ids[i] = <-allocd
	}
	sort.Ints(ids)
	for i := 0; i < 100; i++ {
		if ids[i] != i+2 {
			t.Errorf("expected \"%d\"th ID to be %d; got %d", i, i+2, ids[i])
		}
	}

	// Verify no leftover IDs.
	select {
	case id := <-allocd:
		t.Errorf("there appear to be leftover IDs, starting with %d", id)
	default:
		// Expected; noop.
	}
}

// TestIDAllocatorNegativeValue creates an ID allocator against an
// increment key which is preset to a negative value. We verify that
// the id allocator makes a double-alloc to make up the difference
// and push the id allocation into positive integers.
func TestIDAllocatorNegativeValue(t *testing.T) {
	store, _ := createTestStore(t)
	// Increment our key to a negative value.
	newValue, err := engine.MVCCIncrement(store.Engine(), nil, engine.KeyRaftIDGenerator, store.clock.Now(), nil, -1024)
	if err != nil {
		t.Fatal(err)
	}
	if newValue != -1024 {
		t.Errorf("expected new value to be -1024; got %d", newValue)
	}
	idAlloc, err := NewIDAllocator(engine.KeyRaftIDGenerator, store.db, 2, 10)
	if err != nil {
		t.Errorf("failed to create IDAllocator: %v", err)
	}
	value, err := idAlloc.Allocate()
	if err != nil {
		t.Errorf("failed to allocate id: %v", err)
	}
	if value != 2 {
		t.Errorf("expected id allocation to have value 2; got %d", value)
	}
}

// TestNewIDAllocatorInvalidArgs checks validation logic of NewIDAllocator
func TestNewIDAllocatorInvalidArgs(t *testing.T) {
	args := [][]int64{
		{0, 10}, // minID <= 0
		{2, 0},  // blockSize < 1
	}
	for i := range args {
		if _, err := NewIDAllocator(nil, nil, args[i][0], args[i][1]); err == nil {
			t.Errorf("expect to have error return, but got nil")
		}
	}
}

// TestAllocateErrorHandling creates a invalid IDAllocator which will
// return error when fetch ID from KV DB. Because there isn't existing
// allocated ID, Allocate() will directly return error
func TestAllocateErrorHandling(t *testing.T) {
	store, _ := createTestStore(t)
	// set nil idKey to trigger KV DB increment error
	idAlloc, err := NewIDAllocator(nil, store.db, 2, 10)
	if err != nil {
		t.Errorf("failed to create IDAllocator: %v", err)
	}

	_, err = idAlloc.Allocate()
	if err == nil {
		t.Errorf("expect to return error, but got nil")
	}
}

// TestAllocateErrorWithExistingIDAndRecovery has three steps:
// 1) allocates a set of ID firstly and check
// 2) then makes IDAllocator invalid, error should happen for subsequent call
// 3) set IDAllocator to valid again, can continue to allocate ID
func TestAllocateErrorWithExistingIDAndRecovery(t *testing.T) {
	store, _ := createTestStore(t)

	// firstly create a valid IDAllocator to get some ID
	idAlloc, err := NewIDAllocator(engine.KeyRaftIDGenerator, store.db, 2, 10)
	if err != nil {
		t.Errorf("failed to create IDAllocator: %v", err)
	}

	id, err := idAlloc.Allocate()
	if err != nil {
		t.Errorf("failed to allocate id: %v", err)
	}
	if id != 2 {
		t.Errorf("expected ID is 2, but got: %d", id)
	}

	// set nil idKey to trigger KV DB increment error
	idAlloc.idKey = nil

	// even allocateBlock will return error, but Allocate() will return the
	// existing ID. Already got one ID from channel, and one allocationTrigger
	// in the middle, so there will be only 8 IDs left in the channel, and start
	// from 3
	for i := 0; i < 8; i++ {
		id, err := idAlloc.Allocate()
		if err != nil {
			t.Errorf("failed to allocate id: %v", err)
		}
		if id != int64(i+3) {
			t.Errorf("expected ID is %d, but got: %d", i+3, id)
		}
	}

	// the subsequent Allocate() will return error
	for i := 0; i < 10; i++ {
		_, err := idAlloc.Allocate()
		if err == nil {
			t.Errorf("expect to return error, but got nil")
		}
	}

	// then set correct idKey to recover from error, should be able to allocate
	// ID again
	idAlloc.idKey = engine.KeyRaftIDGenerator
	for i := 11; i < 50; i++ { //previous existing MaxID is 10, so start from 11
		id, err := idAlloc.Allocate()
		if err != nil {
			t.Errorf("failed to allocate id: %v", err)
		}
		if id != int64(i) {
			t.Errorf("expected ID is %d, but got: %d", i, id)
		}
	}
}
