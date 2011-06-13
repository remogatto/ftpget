package ftp

import (
	"fmt"
	"net"
	"os"
	"http"
	"strings"
	"strconv"
	"log"
	"bufio"
	"regexp"
	"path"
)

var (
	DefaultPort = 21
	Log = false
)

type Error struct {
	Code int
	Message string
}

type parsedURL struct {
	addr, path, filename string
}

type response struct {
	code      int
	message   string
	raw       string
	multiline bool
	err os.Error
}

type command struct {
	conn     net.Conn
	response chan *response
	cmd      string
	code     int
}

func (response *response) String() string {
	return fmt.Sprintf("[%03d] %s", response.code, response.message)
}

func (err *Error) String() string {
	return fmt.Sprintf("[%03d] %s", err.Code, err.Message)
}

func parseResponse(r *bufio.Reader) (*response, os.Error) {
	var err os.Error

	multiline := false
	raw, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	code, err := strconv.Atoi(raw[:3])
	if err != nil {
		return nil, err
	}
	message := raw[4 : len(raw)-2]
	if raw[3 : len(raw)-2][0] == '-' {
		multiline = true
	}
	return &response{code, message, raw, multiline, nil}, nil
}

func parseURL(url string) (*parsedURL, os.Error) {
	var (
		urlWithScheme *http.URL
		err           os.Error
	)

	parsedURL := new(parsedURL)

	if urlWithScheme, err = http.ParseURL("ftp://" + url); err != nil {
		return nil, err
	}
	if len(strings.Split(urlWithScheme.Host, ":", -1)) != 2 {
		port := strconv.Itoa(DefaultPort)
		parsedURL.addr = urlWithScheme.Host + ":" + port
	} else {
		parsedURL.addr = urlWithScheme.Host
	}
	parsedURL.filename = path.Base(urlWithScheme.Path)
	parsedURL.path = urlWithScheme.Path[:len(urlWithScheme.Path)-len(parsedURL.filename)]

	return parsedURL, nil
}

func readResponse(conn net.Conn, responseCh chan *response, code int) os.Error {
	reader := bufio.NewReader(conn)
	for {
		response, err := parseResponse(reader)
		if err != nil {
			return err
		}
		if !response.multiline {
			if response.code == code {
				responseCh <- response
				break
			} else {
				return &Error{response.code, response.message}
			}
		}
	}
	return nil
}

func request(cmd *command) os.Error {
	if cmd.cmd != "connect" {
		_, err := cmd.conn.Write([]byte(cmd.cmd + "\r\n"))
		if err != nil {
			return err
		}
	}
	if err := readResponse(cmd.conn, cmd.response, cmd.code); err != nil {
		return err
	}
	return nil
}

func getIpPort(resp string) (addr string, err os.Error) {
	portRegex := "([0-9]+,[0-9]+,[0-9]+,[0-9]+),([0-9]+,[0-9]+)"
	re, err := regexp.Compile(portRegex)
	if err != nil {
		return "", err
	}
	match := re.FindStringSubmatch(resp)
	if len(match) != 3 {
		msg := "Cannot handle server response: " + resp
		return "", os.NewError(msg)
	}

	ip := strings.Replace(match[1], ",", ".", -1)

	octets := strings.Split(match[2], ",", 2)
	firstOctet, _ := strconv.Atoui(octets[0])
	secondOctet, _ := strconv.Atoui(octets[1])
	port := firstOctet*256 + secondOctet
	addr = ip + ":" + strconv.Uitoa(port)

	return addr, nil
}

// A loop that continously listen to requests from the client
// forwarding them to the FTP server through the tcp connection.
func requestLoop(ch chan *command) {
	var err os.Error
	for err == nil {
		select {
		case command := <-ch:
			if err = request(command); err != nil {
				command.response <- &response{err: err}
				// return
			}
		}
	}
}

func connect(addr string) (net.Conn, os.Error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func writeToFile(f *os.File, conn net.Conn) os.Error {
	// Buffer for downloading and writing to file
	bufLen := 1024
	buf := make([]byte, bufLen)
	// Read from the server and write the contents to a file
	for {
		bytesRead, err := conn.Read(buf)
		if bytesRead > 0 {
			_, err := f.Write(buf[0:bytesRead])
			if err != nil {
				return err
			}
		}
		if err == os.EOF {
			break
		}
	}
	return nil
}

func sendCommand(conn net.Conn, commandCh chan *command, cmd string, code int) (*response, os.Error) {
	var r *response
	responseCh := make(chan *response)
	commandCh <- &command{conn, responseCh, cmd, code}
	if Log {
		log.Printf("==> %s", cmd)
	}
	if r = <-responseCh; r.err != nil {
		if Log {
			log.Printf("<== %s", r.err)
		}
		return nil, r.err
	}
	if Log {
		log.Printf("<== %s", r)
	}
	return r, nil
}

// Fetch a file from an FTP server.
// url is the complete URL of the server without the scheme part, ex: ftp.worldofspectrum.org/a/abc.zip
// dst is the destination path
func Get(url string, dst string) os.Error {
	var (
		response *response
		parsedURL *parsedURL = new(parsedURL)
		conn      net.Conn
		err       os.Error
	)

	commandCh := make(chan *command)
	go requestLoop(commandCh)

	if parsedURL, err = parseURL(url); err != nil {
		return err
	}

	conn, err = connect(parsedURL.addr)
	if err != nil {
		return err
	}

	// Begin command sequence
	if _, err = sendCommand(conn, commandCh, "connect", 220); err != nil { return err }
	if _, err = sendCommand(conn, commandCh, "USER anonymous", 331); err != nil { return err }
	if _, err = sendCommand(conn, commandCh, "PASS ftpget@-", 230); err != nil { return err }
	if _, err = sendCommand(conn, commandCh, "CWD "+parsedURL.path, 250); err != nil { return err }
	if _, err = sendCommand(conn, commandCh, "TYPE i", 200); err != nil { return err }
	if response, err = sendCommand(conn, commandCh, "PASV", 227); err != nil { 
		return err 
	} else {
		retrAddr, _ := getIpPort(response.message)
		dataConn, _ := connect(retrAddr)
		if err != nil {
			return err
		}
		if _, err = sendCommand(conn, commandCh, "RETR "+parsedURL.filename, 150); err != nil { 
			return err
		} else {
			f, _ := os.Create(dst)
			writeToFile(f, dataConn)
		}
	}
	return nil
}
