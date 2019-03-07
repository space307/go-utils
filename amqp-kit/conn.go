package amqp_kit

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

const defaultChannelPoolSize = 10
const defaultChannelRetryCount = 5

type connectionCloseHandler interface {
	onCloseWithErr(conn *connection, err error)
}

type connection struct {
	config       *Config
	amqpConn     *amqp.Connection
	pool         *pool
	poolLock     sync.RWMutex
	closeHandler connectionCloseHandler
}

func newConnection(config *Config, closeHandler connectionCloseHandler) *connection {
	conn := &connection{
		config:       config,
		closeHandler: closeHandler,
	}
	return conn
}

func (c *connection) connect() error {
	c.poolLock.Lock()
	defer c.poolLock.Unlock()

	amqpConn, err := amqp.Dial(MakeDsn(c.config))
	if err != nil {
		return err
	}
	c.amqpConn = amqpConn
	poolSize := c.config.ChannelPoolSize
	if poolSize == 0 {
		poolSize = defaultChannelPoolSize
	}

	c.pool = newPool(c.amqpConn, poolSize)
	notifyChan := make(chan *amqp.Error)
	amqpConn.NotifyClose(notifyChan)
	go func() {
		e := <-notifyChan
		log.Errorf(`err notify: %s`, e)
		// clear pool
		c.clearPool()
		// init new connection
		c.closeHandler.onCloseWithErr(c, e)
	}()

	return nil
}

func (c *connection) getChan() (*channel, error) {
	var (
		channel *channel
		err     error
	)

	retryCount := c.config.ChannelRetryCount
	if retryCount == 0 {
		retryCount = defaultChannelRetryCount
	}

	for i := 0; i < defaultChannelRetryCount; i++ {
		c.poolLock.RLock()
		channel, err = c.pool.get()
		c.poolLock.RUnlock()
		if err == nil {
			break
		} else {
			log.Warnf("AMQP: got a channel with err %s", err.Error())
		}
	}
	return channel, err
}

func (c *connection) putChan(channel *channel) {
	c.poolLock.RLock()
	p := c.pool
	c.poolLock.RUnlock()
	if p != nil {
		p.put(channel)
	} else {
		channel.close()
	}
}

func (c *connection) clearPool() {
	c.poolLock.Lock()
	defer c.poolLock.Unlock()

	c.pool.clear()
}

func (c *connection) close() error {
	c.poolLock.Lock()
	defer c.poolLock.Unlock()

	c.pool.clear()
	return c.amqpConn.Close()
}
