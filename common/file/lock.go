package file

import (
	"math"
	"os"
	"time"

	"github.com/nightlyone/lockfile"
	log "github.com/sirupsen/logrus"
)

// Lock attempts to create a file-based lock using the provided file path.
func Lock(lockFile string, maxAttempts int) (*lockfile.Lockfile, error) {
	lock, err := lockfile.New(lockFile)
	if err != nil {
		return nil, err
	}
	i := 0
	for {
		err = lock.TryLock()
		if err == nil {
			return &lock, nil
		}
		if err != nil {
			log.Infof("failed to acquire lock %v: %v", lock, err)
		}
		if err == lockfile.ErrBusy {
			currentOwner, err := lock.GetOwner()
			if err != nil {
				if err == lockfile.ErrDeadOwner {
					log.Infof("current owner of %v is dead, deleting file: %v", lock, err)
					err = os.Remove(lockFile)
					if err != nil {
						log.Errorf("failed to delete file %v: %v", lock, err)
						return &lock, err
					}
					continue
				} else {
					log.Warnf("failed to check current owner of %v: %v", lock, err)
				}
			}
			log.Infof("lock %v currently owned by process #%v", lock, currentOwner.Pid)
		}
		if i < maxAttempts {
			sleepingTime := time.Duration(math.Pow(float64(2), float64(i))) * time.Second
			i++
			log.Infof("exponentially retrying in %v", sleepingTime)
			time.Sleep(sleepingTime)
		} else {
			return &lock, err
		}
	}
}
