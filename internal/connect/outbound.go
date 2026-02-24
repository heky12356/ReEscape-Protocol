package connect

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"project-yume/internal/utils"
)

const (
	outboundQueueSize    = 256
	outboundEnqueueLimit = 200 * time.Millisecond
)

var errOutboundClosed = errors.New("outbound writer is closed")

type queuedMessage struct {
	messageType int
	payload     []byte
	result      chan error
}

type outboundWriter struct {
	queue    chan queuedMessage
	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
}

func newOutboundWriter() *outboundWriter {
	return &outboundWriter{
		queue:  make(chan queuedMessage, outboundQueueSize),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

func (w *outboundWriter) stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
}

func (w *outboundWriter) run(conn *websocket.Conn) {
	defer close(w.doneCh)

	for {
		select {
		case msg := <-w.queue:
			err := conn.WriteMessage(msg.messageType, msg.payload)
			msg.result <- err
			close(msg.result)
		case <-w.stopCh:
			for {
				select {
				case msg := <-w.queue:
					msg.result <- errOutboundClosed
					close(msg.result)
				default:
					return
				}
			}
		}
	}
}

// 按连接维护对应的单写协程实例。
var outboundWriters sync.Map // map[*websocket.Conn]*outboundWriter

func ensureOutboundWriter(conn *websocket.Conn) *outboundWriter {
	if conn == nil {
		return nil
	}

	if value, ok := outboundWriters.Load(conn); ok {
		return value.(*outboundWriter)
	}

	writer := newOutboundWriter()
	actual, loaded := outboundWriters.LoadOrStore(conn, writer)
	if loaded {
		return actual.(*outboundWriter)
	}

	go writer.run(conn)
	return writer
}

// WriteMessage 将消息放入队列，并由连接对应的单写协程串行写出。
func WriteMessage(conn *websocket.Conn, messageType int, payload []byte) error {
	writer := ensureOutboundWriter(conn)
	if writer == nil {
		return errors.New("websocket connection is nil")
	}

	select {
	case <-writer.stopCh:
		return errOutboundClosed
	default:
	}

	req := queuedMessage{
		messageType: messageType,
		payload:     append([]byte(nil), payload...),
		result:      make(chan error, 1),
	}

	select {
	case writer.queue <- req:
	case <-time.After(outboundEnqueueLimit):
		return fmt.Errorf("outbound queue is full")
	case <-writer.stopCh:
		return errOutboundClosed
	}

	select {
	case err := <-req.result:
		return err
	case <-writer.stopCh:
		return errOutboundClosed
	}
}

// Close 先停止该连接的单写协程，再关闭底层 websocket 连接。
func Close(conn *websocket.Conn) error {
	if conn == nil {
		return nil
	}

	if value, ok := outboundWriters.Load(conn); ok {
		writer := value.(*outboundWriter)
		writer.stop()
		<-writer.doneCh
		outboundWriters.Delete(conn)
	}

	err := conn.Close()
	if err != nil && !errors.Is(err, websocket.ErrCloseSent) {
		utils.Warn("close websocket failed: %v", err)
		return err
	}

	return nil
}
