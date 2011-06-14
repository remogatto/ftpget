package ftp

import (
	"testing"
	"os"
)

func TestParseURL(t *testing.T) {
	var (
		err       os.Error
		parsedURL *parsedURL
	)
	if parsedURL, err = parseURL("ftp.worldofspectrum.org/a/b/c.zip"); err != nil {
		t.Error("Parsing url should not fail")
	}
	if parsedURL.addr != "ftp.worldofspectrum.org:21" {
		t.Errorf("Error in parsing the URL. Address should be 'ftp.worldofspectrum.org:21' but is '%s'", parsedURL.addr)
	}
	if parsedURL.path != "/a/b/" {
		t.Errorf("Error in parsing the URL. Path should be '/a/b/' but is '%s'", parsedURL.path)
	}
	if parsedURL.filename != "c.zip" {
		t.Errorf("Error in parsing the URL. Path should be 'c.zip' but is '%s'", parsedURL.filename)
	}
	if parsedURL, err = parseURL("ftp.worldofspectrum.org:1234/a/b/c.zip"); err != nil {
		t.Error("Parsing url with port number should not fail")
	}
	if parsedURL.addr != "ftp.worldofspectrum.org:1234" {
		t.Errorf("Error in parsing the URL. Address should be 'ftp.worldofspectrum.org:1234' but is '%s'", parsedURL.addr)
	}
}

func TestGet(t *testing.T) {
	filename := "AlterEgo.tap.zip"
	expectedSize := 13933
	f, _ := os.Create(filename)
	if err := Get("ftp.worldofspectrum.org/pub/sinclair/games/a/AlterEgo.tap.zip", f); err != nil {
		t.Errorf("Get should not fail with %s", err)
	}
	if fileInfo, err := os.Stat(filename); err != nil {
		t.Errorf("AlterEgo.tap.zip has not been downloaded", err)
	} else if fileInfo.Size != 13933 {
		t.Errorf("AlterEgo.tap.zip has the wrong expected size %d", expectedSize)
	} else {
		os.Remove(filename)
	}
}

func TestGetAsynch(t *testing.T) {
	filename := "AlterEgo.tap.zip"
	expectedSize := 13933
	f, _ := os.Create(filename)

	transfer, err := GetAsync("ftp.worldofspectrum.org/pub/sinclair/games/a/AlterEgo.tap.zip", f)
	if err != nil {
		t.Error("GetAsync should not return error")
	}
	if status := <-transfer.Status; status != STARTED {
		t.Error("Status should be STARTED")
	}
	if status := <-transfer.Status; status != COMPLETED {
		t.Error("Status should be COMPLETED")
	}
	if fileInfo, err := os.Stat(filename); err != nil {
		t.Errorf("AlterEgo.tap.zip has not been downloaded", err)
	} else if fileInfo.Size != 13933 {
		t.Errorf("AlterEgo.tap.zip has the wrong expected size %d", expectedSize)
	} else {
		os.Remove(filename)
	}
}

func TestGetAsynchAbort(t *testing.T) {
	filename := "ubuntu-11.04-desktop-i386.iso"
	f, _ := os.Create(filename)

	transfer, err := GetAsync("ftp.ussg.iu.edu/linux/ubuntu-releases/natty/ubuntu-11.04-desktop-i386.iso", f)
	if err != nil {
		t.Error("GetAsync should not return error")
	}
	if status := <-transfer.Status; status != STARTED {
		t.Error("Status should be STARTED")
	}

	transfer.Control <- ABORT

	if status := <-transfer.Status; status != ABORTED {
		t.Error("Status should be ABORTED but is %d", status)
	}
	if fileInfo, _ := os.Stat(filename); fileInfo.Size > 100000000 {
		t.Errorf("The iso image size should be < 100MB but is %d", fileInfo.Size)
	}
	os.Remove(filename)
}

func TestErrorHandling(t *testing.T) {
	if err := Get("doesntexist/pub/sinclair/games/a/b.tap.zip", nil); err == nil {
		t.Error("Should fail")
	}
	if err := Get("ftp.worldofspectrum.org/pub/sinclair/games/a/b.tap.zip", nil); err != nil {
		ftpErr := err.(*Error)
		if ftpErr.Code != 550 {
			t.Errorf("Error code should be 550 but is %d", ftpErr.Code)
		}
	}
}
