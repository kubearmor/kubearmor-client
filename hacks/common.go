package hacks

import (
	"os"

	log "github.com/sirupsen/logrus"
)

func CloseCheckErr(f *os.File, fname string) {
	err := f.Close()
	if err != nil {
		log.WithFields(log.Fields{
			"file": fname,
		}).Error("close file failed")
	}
}
