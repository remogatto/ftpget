# What's that?

<tt>ftpget</tt> is a simple ftp fetcher library written in just ~300
lines of Go code.

# Example

<pre>
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
</pre>

# Install

<pre>
goinstall github.com/remogatto/ftpget
</pre>

# License

opyright (c) 2011 Andrea Fazzi

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
