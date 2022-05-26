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

## License

[BSD](LICENSE)
