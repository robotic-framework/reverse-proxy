package worker

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"github.com/eden-framework/context"
	"github.com/profzone/envconfig"
	"github.com/robotic-framework/reverse-proxy/common"
	"io"
	"net"
	"sync/atomic"
	"time"
)

type Worker struct {
	RemoteAddr    string
	RetryInterval envconfig.Duration
	RetryMaxTime  int

	sequence uint32
	r        *Router
	ctx      *context.WaitStopContext
}

func (w *Worker) Init() {
	w.setDefaults()
	w.r = NewRouter()
}

func (w *Worker) setDefaults() {
	if w.RetryMaxTime == 0 {
		w.RetryMaxTime = 5
	}
	if w.RetryInterval == 0 {
		w.RetryInterval = envconfig.Duration(time.Second)
	}
}

func (w *Worker) Start(ctx *context.WaitStopContext) {
	if ctx == nil {
		ctx = context.NewWaitStopContext()
		w.ctx = ctx
	}
	ctx.Add(1)
	defer ctx.Finish()

	conn, err := net.Dial("tcp4", w.RemoteAddr)
	if err != nil {
		panic(err)
	}

	go w.handleMasterConn(ctx, conn)

	writer := bufio.NewWriter(conn)
	err = w.handshake(writer)

	<-ctx.Done()
	w.r.Close()
	_ = conn.Close()
}

func (w *Worker) Stop() {
	if w.ctx != nil {
		w.ctx.Cancel()
	}
}

func (w *Worker) writePacket(writer io.Writer, p *common.Packet) error {
	if p.Sequence == 0 {
		atomic.AddUint32(&w.sequence, 1)
		p.Sequence = w.sequence
	}
	packetBytes, err := p.MarshalBinary()
	if err != nil {
		return err
	}
	packetLengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(packetLengthBytes, uint32(len(packetBytes)))

	buf := bytes.NewBuffer([]byte{})
	buf.WriteString(common.PacketBytesPrefix)
	buf.Write(packetLengthBytes)
	buf.Write(packetBytes)

	_, err = writer.Write(buf.Bytes())
	return err
}

func (w *Worker) AddRoute(remotePort int, handler Handler) {
	w.r.AddRoute(remotePort, handler)
}
