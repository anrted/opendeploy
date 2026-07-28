//go:build windows

package update

import (
	"fmt"
	"sync"
)

var (
	processLock sync.Mutex
	lockHeld    bool
)

func acquireUpdateLock(_ string) (func(), error) {
	processLock.Lock()
	defer processLock.Unlock()
	if lockHeld {
		return nil, fmt.Errorf("update: another update or rollback is in progress")
	}
	lockHeld = true
	return func() {
		processLock.Lock()
		lockHeld = false
		processLock.Unlock()
	}, nil
}
