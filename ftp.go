package ftp

import (
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

type parsedURL struct {
	addr, path, filename string
}

type response struct {
	code      int
	message   string
	raw       string
	multiline bool
}

type command struct {
	conn     net.Conn
	response chan *response
	cmd      string
	code     int
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
	return &response{code, message, raw, multiline}, nil
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

func request(cmd *command) os.Error {
	if cmd.cmd != "connect" {
		_, err := cmd.conn.Write([]byte(cmd.cmd + "\r\n"))
		if err != nil {
			return err
		}
	}
	reader := bufio.NewReader(cmd.conn)
	for {
		response, err := parseResponse(reader)
		if err != nil {
			return err
		}
		if Log {
			log.Print(response.raw)
		}
		if response.code == cmd.code && !response.multiline {
			cmd.response <- response
			break
		}
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

func commandLoop(ch chan *command) {
	for {
		select {
		case command := <-ch:
			request(command)
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

func Get(url string, dst string) os.Error {
	var (
		parsedURL *parsedURL = new(parsedURL)
		conn      net.Conn
		err       os.Error
	)

	commandCh := make(chan *command)
	go commandLoop(commandCh)

	if parsedURL, err = parseURL(url); err != nil {
		return err
	}

	conn, err = connect(parsedURL.addr)
	if err != nil {
		return err
	}

	response := make(chan *response)

	commandCh <- &command{conn, response, "connect", 220}
	<-response
	commandCh <- &command{conn, response, "USER anonymous", 331}
	<-response
	commandCh <- &command{conn, response, "PASS ftpget@-", 230}
	<-response
	commandCh <- &command{conn, response, "CWD "+parsedURL.path, 250}
	<-response
	commandCh <- &command{conn, response, "TYPE I", 200}
	<-response
	commandCh <- &command{conn, response, "PASV", 227}
	retrAddr, _ := getIpPort((<-response).message)

	dataConn, _ := connect(retrAddr)
	if err != nil {
		return err
	}
	f, _ := os.Create(dst)
	commandCh <- &command{conn, response, "RETR "+parsedURL.filename, 150}
	<-response

	// Buffer for downloading and writing to file
	bufLen := 1024
	buf := make([]byte, bufLen)
	// Read from the server and write the contents to a file
	for {
		bytesRead, err := dataConn.Read(buf)
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
