package filemanager

import "sync"

// FileLockManager provides per-file RWMutex locking.
// Multiple readers can hold a lock simultaneously, but writers get exclusive access.
type FileLockManager struct {
	mu    sync.Mutex
	locks map[string]*sync.RWMutex
}

func NewFileLockManager() *FileLockManager {
	return &FileLockManager{
		locks: make(map[string]*sync.RWMutex),
	}
}

func (flm *FileLockManager) getLock(path string) *sync.RWMutex {
	flm.mu.Lock()
	defer flm.mu.Unlock()
	lock, ok := flm.locks[path]
	if !ok {
		lock = &sync.RWMutex{}
		flm.locks[path] = lock
	}
	return lock
}

func (flm *FileLockManager) RLock(path string) {
	flm.getLock(path).RLock()
}

func (flm *FileLockManager) RUnlock(path string) {
	flm.getLock(path).RUnlock()
}

func (flm *FileLockManager) Lock(path string) {
	flm.getLock(path).Lock()
}

func (flm *FileLockManager) Unlock(path string) {
	flm.getLock(path).Unlock()
}
