package ftp

import (
	"fmt"
	"net"
	"os"
	"http"
	"strings"
	"strconv"
	"log"
	"io"
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
	cmd      string
	code     int
	response *response
}

func (*command) newCommand(conn net.Conn, cmd string, code int) *command {
	return &command{conn, cmd, code, new(response)}
}

func (response *response) String() string {
	return fmt.Sprintf("[%03d] %s", response.code, response.message)
}

func (err *Error) String() string {
	return fmt.Sprintf("[%03d] %s", err.Code, err.Message)
}

func parseResponse(r *bufio.Reader) *response {
	var (
		code int
		message string
		raw string
		multiline bool
		err os.Error
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

func parseURL(url string) (*parsedURL, os.Error) {
	var (
		urlWithScheme *http.URL
		parsedURL *parsedURL = new(parsedURL)
		err           os.Error
	)
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

func readResponse(conn net.Conn, code int) *response {
	var response *response
	reader := bufio.NewReader(conn)
	for {
		if response = parseResponse(reader); response.err != nil {
			return response
		} else {
			if !response.multiline {
				if response.code == code {
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
	var err os.Error
	if cmd.cmd != "connect" {
		_, err = cmd.conn.Write([]byte(cmd.cmd + "\r\n"))
		if err != nil {
			return &response{err: err}
		}
	}
	return readResponse(cmd.conn, cmd.code)
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

func connect(addr string) (net.Conn, os.Error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func sendCommand(conn net.Conn, cmd string, code int) (*response, os.Error) {
	response := request(&command{conn, cmd, code, nil})
	if Log {
		log.Printf("==> %s", cmd)
	}
	if response.err != nil {
		if Log {
			log.Printf("<== %s", response.err)
		}
		return nil, response.err
	}
	if Log {
		log.Printf("<== %s", response)
	}
	return response, nil
}

func sendCommandSequence(conn net.Conn, parsedURL *parsedURL, w io.Writer) os. Error {
	if _, err := sendCommand(conn, "connect", 220); err != nil { return err }
	if _, err := sendCommand(conn, "USER anonymous", 331); err != nil { return err }
	if _, err := sendCommand(conn, "PASS ftpget@-", 230); err != nil { return err }
	if _, err := sendCommand(conn, "CWD "+parsedURL.path, 250); err != nil { return err }
	if _, err := sendCommand(conn, "TYPE I", 200); err != nil { return err }
	if response, err := sendCommand(conn, "PASV", 227); err != nil { 
		return err 
	} else {
		retrAddr, _ := getIpPort(response.message)
		dataConn, _ := connect(retrAddr)
		if err != nil {
			return err
		}
		if _, err = sendCommand(conn, "RETR "+parsedURL.filename, 150); err != nil { 
			return err
		} else {
			_, err := io.Copy(w, dataConn)
			return err
		}
	}
	return nil
}

// Fetch a file from an FTP server.
// url is the complete URL of the FTP server without the scheme part, ex: ftp.worldofspectrum.org/a/abc.zip
// w is an object that implements the io.Writer interface
func Get(url string, w io.Writer) os.Error {
	if parsedURL, err := parseURL(url); err != nil {
		return err
	} else {
		if conn, err := connect(parsedURL.addr); err != nil {
			return err
		} else {
			if err := sendCommandSequence(conn, parsedURL, w); err != nil {
				return err
			}
			conn.Close()
		}
	}
	return nil
}
