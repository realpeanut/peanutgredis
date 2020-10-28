/**
 * @Author: realpeanut
 * @Date: 2020/10/27 3:57 下午
 */
package peanutRedis

import (
	"bufio"
	"io"
	"net"
	"strconv"
	"strings"
)

//状态回复（status reply）的第一个字节是 "+"
//
//错误回复（error reply）的第一个字节是 "-"
//
//整数回复（integer reply）的第一个字节是 ":"
//
//批量回复（bulk reply）的第一个字节是 "$"
//
//多条批量回复（multi bulk reply）的第一个字节是 "*" 相当于 $*n

const TCP4 = "tcp4"

type Query struct {
	command string
}

type Conn struct {
	conn *net.TCPConn
}

type redisCli struct {
	Query
	Conn
}

func (rc redisCli) conn(host string, port int16) redisCli {
	tcpAddr, _ := net.ResolveTCPAddr(TCP4, host + ":" + Int16ToString(port))
	rc.Conn.conn, _ = net.DialTCP(TCP4, nil, tcpAddr)
	return rc
}

func (rc redisCli) query(command string) (interface{},error){

	commandSlice := strings.Split(command, " ")
	rc.Query.command = "*" + strconv.Itoa(len(commandSlice)) + "\r\n"

	for _, v := range commandSlice {

		rc.Query.command += "$"
		rc.Query.command += strconv.Itoa(len(v)) + "\r\n"
		rc.Query.command += v + "\r\n"

	}
	_, err := rc.Conn.conn.Write([]byte(rc.Query.command))

	if err != nil {
		return nil,err
	}

	res, err := rc.readLineGetSizeAndReply()
	if res == nil {
		//键不存在
		return nil,err
	}
	defer rc.Conn.conn.Close()

	return res,err
}

func Int16ToString(c int16) string {
	return strconv.FormatInt(int64(c), 10)
}

func (rc redisCli) readLineGetSizeAndReply() (interface{}, error) {
	r := bufio.NewReader(rc.Conn.conn)
	p, err := r.ReadSlice('\n')
	p = p[:len(p)-2]
	if err == bufio.ErrBufferFull {
		buf := append([]byte{}, p...)
		//如果缓存区读完后仍然没有找到\n 则报ErrBufferFull错误，此时继续读取buff直到找到\n
		//如果找到最后任然没有找到\n 则err非空
		for err == bufio.ErrBufferFull {
			p, err = r.ReadSlice('\n')
			buf = append(buf, p...)
		}
		p = buf
	}
	//此时response为$-1或者response格式❌
	if err != nil {
		return nil, nil
	}
	switch p[0] {
	case '+':
		switch string(p[1:]) {
		case "OK":

			return []byte("OK"), nil
		case "PONG":

			return []byte("PONG"), nil
		default:
			return p[1:], nil
		}
	case '-':
		return p[1:], nil
	case ':':
		return parseInt(p[1:])
	case '$':
		//减去 $ 和\r两个字节  例如 $5\r\n 此时p为$5\r p[i] == \r
		i := len(p) - 2
		res := p[:i]
		var n int
		for _, b := range res[1:] {
			n *= 10
			n += int(b - '0')
		}
		rs := make([]byte, n)
		_, _ = io.ReadFull(r, rs)
		return rs,nil
	case '*':
		n, err := parseLen(p[1:])
		if n < 0 || err != nil {
			return nil, err
		}
		r := make([]interface{}, n)
		for i := range r {
			r[i], err = rc.readLineGetSizeAndReply()
			if err != nil {
				return nil, err
			}
		}
		return r, nil
	}
	return nil,nil
}

func parseLen(p []byte) (int, error) {
	if len(p) == 0 {
		return -1, nil
	}

	if p[0] == '-' && len(p) == 2 && p[1] == '1' {
		// handle $-1 and $-1 null replies.
		return -1, nil
	}

	var n int
	for _, b := range p {
		n *= 10
		if b < '0' || b > '9' {
			return -1, nil
		}
		n += int(b - '0')
	}

	return n, nil
}

func parseInt(p []byte) (interface{}, error) {
	if len(p) == 0 {
		return 0, nil
	}

	var negate bool
	if p[0] == '-' {
		negate = true
		p = p[1:]
		if len(p) == 0 {
			return 0, nil
		}
	}

	var n int64
	for _, b := range p {
		n *= 10
		if b < '0' || b > '9' {
			return 0, nil
		}
		n += int64(b - '0')
	}

	if negate {
		n = -n
	}
	return n, nil
}
