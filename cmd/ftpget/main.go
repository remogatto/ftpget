package main

import (
	"flag"
	"fmt"
	"github.com/remogatto/ftpget"
	"log"
	"os"
	"path"
	"sync"
)

func main() {
	log.SetFlags(0)

	help := flag.Bool("help", false, "Show usage")
	verbose := flag.Bool("verbose", false, "Verbose output")
	async := flag.Bool("async", false, "Asynchronous transfers")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "FTP fetcher\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "\tftpget [OPTIONS] URLS\n\n")
		fmt.Fprintf(os.Stderr, "\tExample URL: ftp.gnu.org/gnu/bash/bash-4.2.tar.gz\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *help == true {
		flag.Usage()
		return
	}
	if *verbose == true {
		ftp.Log = true
	}
	if url := flag.Arg(0); url != "" {
		if !*async {
			paths := flag.Args()
			for _, p := range paths {
				if f, err := os.Create(path.Base(p)); err != nil {
					log.Fatal(err)
				} else {
					err = ftp.Get(p, f)
					if err != nil {
						os.Remove(f.Name())
						log.Fatal(err)
					}
					f.Close()
					if *verbose {
						log.Printf("Transfer of '%s' complete", p)
					}
				}
			}
		} else {
			failed := false

			paths := flag.Args()
			var wg sync.WaitGroup
			for _, p := range paths {
				wg.Add(1)
				go func(p string) {
					file, err := os.Create(path.Base(p))
					if err != nil {
						log.Println(err)
						failed = true
						wg.Done()
						return
					}
					defer file.Close()

					transfer, err := ftp.GetAsync(p, file)
					if err != nil {
						os.Remove(file.Name())
						log.Println(err)
						failed = true
						wg.Done()
						return
					}

					switch <-transfer.Status {
					case ftp.STARTED:
						log.Printf("Start download of '%s'", p)
						switch <-transfer.Status {
						case ftp.COMPLETED:
							log.Printf("Download of '%s' completed", p)
						case ftp.ABORTED:
							log.Printf("Download of '%s' was aborted", p)
							os.Remove(file.Name())
							failed = true
						case ftp.ERROR:
							log.Printf("Error while downloading '%s': %s", p, <-transfer.Error)
							os.Remove(file.Name())
							failed = true
						}
					}
					wg.Done()
				}(p)
			}
			wg.Wait()

			if failed {
				os.Exit(1)
			}
		}
	} else {
		flag.Usage()
		return
	}
}
