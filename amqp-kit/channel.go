package amqp_kit

import "github.com/streadway/amqp"

type pool struct {
	ch chan *channel
	c  *amqp.Connection
}

type channel struct {
	c   *amqp.Channel
	err error
}

func (c *channel) close() {
	if c != nil && c.c != nil {
		c.c.Close()
	}
}

func newPool(c *amqp.Connection, size int) *pool {
	return &pool{
		c:  c,
		ch: make(chan *channel, size),
	}
}

func (p *pool) get() (*channel, error) {
	select {
	case conn := <-p.ch:
		return conn, nil
	default:
		c := &channel{}
		c.c, c.err = p.c.Channel()
		return c, c.err
	}
}

func (p *pool) put(c *channel) {
	if c.err != nil {
		return
	}

	select {
	case p.ch <- c:
	default:
		c.close()
	}
}

func (p *pool) empty() {
	var c *channel
	for {
		select {
		case c = <-p.ch:
			c.close()
		default:
			return
		}
	}
}
