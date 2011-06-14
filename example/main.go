package main

import (
	"os"
	"fmt"
	"flag"
	"path"
	"log"
	"github.com/remogatto/ftpget"
)

func main() {
	help := flag.Bool("help", false, "Show usage")
	verbose := flag.Bool("verbose", false, "Be verbose!")
	async := flag.Bool("async", false, "Asynchronous transfers")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "FTP fetcher\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n\n")
		fmt.Fprintf(os.Stderr, "\tftpget ftp.foo.bar/a/ab/abc.zip\n")
		fmt.Fprintf(os.Stderr, "\tftpget -async ftp.foo.bar/a/ab/ abc.zip abcde.zip\n\n")
		fmt.Fprintf(os.Stderr, "Options are:\n\n")
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
			if f, err := os.Create(path.Base("ftp://"+flag.Arg(0))); err != nil {
				panic(err)
			} else {
				ftp.Get(flag.Arg(0), f)
				if *verbose {
					log.Print("Transfer complete")
				}
			}
		} else {
			basePath := flag.Arg(0)
			filenames := flag.Args()[1:len(flag.Args())]
			n := len(filenames)
			transfers := make([]*ftp.Transfer, n)
			for i, fn := range filenames {
				if w, err := os.Create(fn); err != nil {
					panic(err)
				} else {
					if transfers[i], err = ftp.GetAsync(path.Join(basePath, fn), w); err != nil {
						panic(err)
					}
				}
			}
			for i, t := range transfers {
				if <-t.Status == ftp.STARTED {
					log.Printf("Start download of '%s'", filenames[i])
				}
				if <-t.Status == ftp.COMPLETED {
					log.Printf("Download of '%s' completed", filenames[i])
				}
			}
		}
	} else {
		flag.Usage()
		return
	}

}
