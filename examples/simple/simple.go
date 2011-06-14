package main

import (
	"fmt"
	"os"
	"github.com/remogatto/ftpget"
)

func main() {
	ftp.Log = true
	f, _ := os.Create("EarthAttack.tap.zip")

	// Synchronous file transfer
	if err := ftp.Get("ftp.worldofspectrum.org/pub/sinclair/games/e/EarthAttack.tap.zip", f); err != nil {
		panic(err)
	} else {
		fmt.Println("Transfer completed")
	}

	f, _ = os.Create("Eagle.tap.zip")

	// ASynchronous file transfer

	// GetAsync spawns the fetching routine but doesn't wait for
	// the transfer to finish. It returns a Transfer object in
	// order to control the transfer status through channels.
	if transfer, err := ftp.GetAsync("ftp.worldofspectrum.org/pub/sinclair/games/e/Eagle.tap.zip", f); err != nil {
		panic(err)
	} else {
		// Control the transfer status and errors.
		// The transfer state diagram is:
		// STARTED --> COMPLETED
		//         |
		//         --> ABORTED
		//         |
		//         --> ERROR (in this case you should drain the Error channel)
		//
		if status := <-transfer.Status; status == ftp.STARTED {
			if status = <-transfer.Status; status == ftp.COMPLETED {
				fmt.Println("Transfer completed")
			} else if status == ftp.ERROR {
				panic(<-transfer.Error)
			} else {
				panic("Unknown status")
			}
		}
	}
}
