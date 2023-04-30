# TRU v5

[Teonet](https://github.com/teonet-go/teonet) Real time communications over UDP protocol (TRU)

[![GoDoc](https://godoc.org/github.com/teonet-go/tru?status.svg)](https://godoc.org/github.com/teonet-go/tru/)
[![Go Report Card](https://goreportcard.com/badge/github.com/teonet-go/tru)](https://goreportcard.com/report/github.com/teonet-go/tru)

The TRU protocol is the UDP based protocol for real time communications that allows sending messages with low latency and provides protocol reliability features.

Low latency

- Send application message to its destination as soon as possible
- Receive and deliver message to application as soon as possible

Reliability

- Avoid losing messages
- In case of lost message, detect and recover it (time that takes to recover a message is related to both reliability and low latency requirements, so the protocol should pay a special attention to this issue)
- Handle messages reordering

## Usage examples

The Tru used in [Teonet](https://github.com/teonet-go/teonet) as it transport protocol. There is two basic examples to show how to use the TRU protocol in any golang application.

### Tru native example

Its Client server application. Server starts listen messages at selected in app parameters port (-p), when message receivet server replay with simple answer message. Client starts send messages to addres and port selected in app parameter (-a). To get all applcation parameters use -? or -help flag.

The `Tru native example` uses most of tru native methods to transfer nessages between connected peers.

start server app:

    go run ./examples/tru -p 7070 -stat -loglevel=debug

start client app (you can starts any number of clients):

    go run ./examples/tru -a localhost:7070 -stat -loglevel=debug

Any Tru Library connections may be client or server. We use the terms client/server just to clear explain what this samples application do. It well be true to call this applications peer-1 and peer-2.

### Tru golang net example

Its Client server application which transfer data using golang net standard function.

start server app:

    go run ./examples/trunet/

start client app:

    go run ./examples/trunet/ -a :7070

In this example You can use go run tags parameter to show tru statistic and
debug messages. Can use -nomsg application parameter to switch off message print.

start server app with tru statistic:

    go run -tags=stat ./examples/trunet/

start server app with tru statistic:

    go run -tags=debug ./examples/trunet/

start server app with tru statistic, bebugvv messages and without app messages:

    go run -tags=debug,stat ./examples/trunet/ -nomsg

## License

[BSD](LICENSE)
