package fsops

import (
	"os"
	"sync"

	"github.com/petasbytes/go-agent/internal/safety"
)

var (
	rootsOnce    sync.Once
	absReadRoot  string
	absWriteRoot string
	initRootsErr error
)

func initRoots() {
	read := os.Getenv("AGT_READ_ROOT")
	write := os.Getenv("AGT_WRITE_ROOT")
	absReadRoot, absWriteRoot, initRootsErr = safety.InitSandboxRoot(read, write)
}

// getRoots returns the cached absolute read/write roots, initialising them once on first use.
func getRoots() (string, string, error) {
	rootsOnce.Do(initRoots)
	return absReadRoot, absWriteRoot, initRootsErr
}
