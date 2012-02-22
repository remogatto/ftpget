package ftp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
)

type Status int
type Control int

// Status
const (
	STARTED Status = iota
	COMPLETED
	ERROR
	ABORTED
)

// Control
const (
	ABORT Control = iota
)

const DefaultPort = 21

var Log = false

type Transfer struct {
	Status  <-chan Status
	Control chan<- Control
	Error   <-chan error
}

type Error struct {
	Code    int
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
	err       error
}

type command struct {
	conn      net.Conn
	cmd       string
	termCodes []int
	response  *response
}

func (*command) newCommand(conn net.Conn, cmd string, termCodes []int) *command {
	return &command{conn, cmd, termCodes, new(response)}
}

func (response *response) String() string {
	return fmt.Sprintf("[%03d] %s", response.code, response.message)
}

func (err *Error) Error() string {
	return fmt.Sprintf("[%03d] %s", err.Code, err.Message)
}

func parseResponse(r *bufio.Reader) *response {
	var (
		code      int
		message   string
		raw       string
		multiline bool
		err       error
	)
	if raw, err = r.ReadString('\n'); err != nil {
		return &response{err: err}
	} else {
		if code, err = strconv.Atoi(raw[:3]); err != nil {
			return &response{err: err}
		}
		message = raw[4 : len(raw)-2]
		if raw[3 : len(raw)-2][0] == '-' {
			multiline = true
		}
	}
	return &response{code, message, raw, multiline, nil}
}

func parseURL(URL string) (*parsedURL, error) {
	var urlWithScheme *url.URL
	var parsedURL parsedURL

	urlWithScheme, err := url.Parse("ftp://" + URL)
	if err != nil {
		return nil, err
	}
	if urlWithScheme.Path == "" {
		return nil, fmt.Errorf("invalid URL: %s", URL)
	}

	parsedURL.addr = urlWithScheme.Host
	if strings.Index(urlWithScheme.Host, ":") == -1 {
		parsedURL.addr += ":" + strconv.Itoa(DefaultPort)
	}

	parsedURL.filename = path.Base(urlWithScheme.Path)
	parsedURL.path = urlWithScheme.Path[:len(urlWithScheme.Path)-len(parsedURL.filename)]
	return &parsedURL, nil
}

func readResponse(conn net.Conn, termCodes []int) *response {
	isTermCode := make(map[int]bool)
	for _, c := range termCodes {
		isTermCode[c] = true
	}

	var response *response
	reader := bufio.NewReader(conn)
	for {
		if response = parseResponse(reader); response.err != nil {
			return response
		} else {
			if !response.multiline {
				if isTermCode[response.code] {
					break
				} else {
					response.err = &Error{response.code, response.message}
					return response
				}
			}
		}
	}
	return response
}

func request(cmd *command) *response {
	var err error
	if cmd.cmd != "connect" {
		_, err = cmd.conn.Write([]byte(cmd.cmd + "\r\n"))
		if err != nil {
			return &response{err: err}
		}
	}
	return readResponse(cmd.conn, cmd.termCodes)
}

func getIpPort(resp string) (addr string, err error) {
	portRegex := "([0-9]+,[0-9]+,[0-9]+,[0-9]+),([0-9]+,[0-9]+)"
	re, err := regexp.Compile(portRegex)
	if err != nil {
		return "", err
	}
	match := re.FindStringSubmatch(resp)
	if len(match) != 3 {
		msg := "Cannot handle server response: " + resp
		return "", errors.New(msg)
	}
	ip := strings.Replace(match[1], ",", ".", -1)
	octets := strings.SplitN(match[2], ",", 2)
	firstOctet, _ := strconv.ParseUint(octets[0], 10, 0)
	secondOctet, _ := strconv.ParseUint(octets[1], 10, 0)
	port := firstOctet*256 + secondOctet
	addr = ip + ":" + strconv.FormatUint(uint64(port), 10)
	return addr, nil
}

func connect(addr string) (net.Conn, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func writeToFile(conn net.Conn, w io.Writer, statusCh chan Status, controlCh chan Control, errCh chan error) {
	bufLen := 32 * 1024
	buf := make([]byte, bufLen)
	statusCh <- STARTED
loop:
	for {
		select {
		case ctrl := <-controlCh:
			switch ctrl {
			case ABORT:
				statusCh <- ABORTED
				break loop
			}
		default:
			bytesRead, err := conn.Read(buf)
			if bytesRead > 0 {
				if _, err2 := w.Write(buf[0:bytesRead]); err != nil {
					statusCh <- ERROR
					errCh <- err2
					break loop
				}
			}
			if err == io.EOF {
				statusCh <- COMPLETED
				break loop
			}
			if err != nil {
				statusCh <- ERROR
				errCh <- err
				break loop
			}
		}
	}
	close(statusCh)
	close(errCh)
}

func sendCommand(conn net.Conn, cmd string, termCodes ...int) (*response, error) {
	response := request(&command{conn, cmd, termCodes, nil})
	if Log {
		log.Printf("==> %s", cmd)
	}
	if response.err != nil {
		if Log {
			log.Printf("<== ERROR %s", response.err)
		}
		return nil, response.err
	}
	if Log {
		log.Printf("<== %s", response)
	}
	return response, nil
}

func sendCommandSequence(conn net.Conn, parsedURL *parsedURL, w io.Writer) (net.Conn, error) {
	var r *response
	var err error
	if r, err = sendCommand(conn, "connect", 220); err != nil {
		return nil, err
	}
	if r, err = sendCommand(conn, "USER anonymous", 230, 331); err != nil {
		return nil, err
	}
	if r.code == 331 {
		if r, err = sendCommand(conn, "PASS ftpget@-", 230); err != nil {
			return nil, err
		}
	}
	if r.code != 230 {
		return nil, fmt.Errorf("invalid response: %s", r.String())
	}
	if r, err = sendCommand(conn, "CWD "+parsedURL.path, 250); err != nil {
		return nil, err
	}
	if r, err = sendCommand(conn, "TYPE I", 200); err != nil {
		return nil, err
	}
	var response *response
	if response, err = sendCommand(conn, "PASV", 227); err != nil {
		return nil, err
	}
	retrAddr, err := getIpPort(response.message)
	if err != nil {
		return nil, err
	}
	dataConn, err := connect(retrAddr)
	if err != nil {
		return nil, err
	}
	return dataConn, nil
}

func get(URL string, w io.Writer, async bool) (*Transfer, error) {
	statusCh, controlCh := make(chan Status), make(chan Control)
	errCh := make(chan error)

	parsedURL, err := parseURL(URL)
	if err != nil {
		return nil, err
	}

	if conn, err := connect(parsedURL.addr); err != nil {
		return nil, err
	} else {
		if dataConn, err := sendCommandSequence(conn, parsedURL, w); err != nil {
			conn.Close()
			return nil, err
		} else {
			if _, err = sendCommand(conn, "RETR "+parsedURL.filename, 150); err != nil {
				dataConn.Close()
				conn.Close()
				return nil, err
			} else {
				if async {
					go func() {
						writeToFile(dataConn, w, statusCh, controlCh, errCh)
						dataConn.Close()
						conn.Close()
					}()
				} else {
					_, err := io.Copy(w, dataConn)
					dataConn.Close()
					conn.Close()
					return nil, err
				}

			}
		}
	}
	return &Transfer{statusCh, controlCh, errCh}, nil
}

// Fetch a file from an FTP server. The transfer process is synchronous.
// URL is the complete URL of the FTP server without the scheme part, ex: ftp.worldofspectrum.org/a/abc.zip
// w is an object that implements the io.Writer interface
func Get(URL string, w io.Writer) error {
	_, err := get(URL, w, false)
	return err
}

// Fetch a file from an FTP server and return a Transfer object in
// order to control the transfer. The transfer process is
// asynchronous.
// The function spawns the fetching routine but doesn't wait for
// the transfer to finish. It returns a Transfer object in
// order to control the transfer status through channels.
//
// The transfer state diagram is:
// STARTED --> COMPLETED
//         |
//         --> ABORTED
//         |
//         --> ERROR (in this case you should drain the Error channel)
//
// URL is the complete URL of the FTP server without the scheme part, ex: ftp.worldofspectrum.org/a/abc.zip
// w is an object that implements the io.Writer interface
func GetAsync(URL string, w io.Writer) (*Transfer, error) {
	return get(URL, w, true)
}
