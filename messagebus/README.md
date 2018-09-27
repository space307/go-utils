## RabbitMQ utils

Wraps [amqp package](http://github.com/streadway/amqp)

Creates exchange, queue, bindings for routing keys automatically

There is three methods:

- Dial(Config) - connect to amqp-message broker.
- Produce(exchange, key, body) - produce message to the given exchange. Declares exchange if not exists
- Consume(exchange, queue, keys, handler) - consumes messages with `handler`. Delcares exchange, queue, make binding
from exchange to queue for given set of keys.