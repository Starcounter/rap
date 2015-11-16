# REST Aggregation Protocol

The REST Aggregation Protocol, or *RAP* for short, is an asymmetric HTTP and WebSocket reverse-proxy multiplexing protocol designed to allow high volume of relatively small request-response exchanges while minimizing the resource usage on the upstream server.

## Motivation

Parsing HTTP and correctly implementing all the features of a HTTP server uses up significant resources on the server. Simply handling the TCP protocol requirements for tens of thousands of connections can consume significant CPU resources on the server.

Traditional web applications simply accept this as an unavoidable fact and focus on finding ways to add more servers. But that brings it's own set of problems and isn't a viable solution for Starcounter.

In Starcounter, it is generally not possible to decouple the application code that processes HTTP requests from the database that holds the data. So everything that runs on the same machine must be as efficient as possble. Unfortunately, even the most efficient web servers today use far too much CPU.

Our solution is to move the web server to other machine(s) and to simplify the request-response scheme to support only what we need while making sure that receiving and routing the incoming requests use as few resources as possible. Where HTTP tries to be very generic in it's design, RAP focuses on handling large amounts of small request-reply exchanges.

We initially looked to HTTP/2 as a likely candidate for multiplexing HTTP requests, but found that it used too much CPU and that the HPACK algorithm made the stream stateful, which meant synchronization mechanisms would be needed in order to use more than one thread per stream on the upstream server.

## Overview

One or more RAP *gateways* are connected to a single upstream server. The gateways receive incoming requests using any protocol it supports (HTTP(S), HTTP/2, SPDY etc) and multiplexes these onto one or more RAP *connections*. The gateways need no configuration data except for the upstream destination address.

A RAP *connection* multiplexes concurrent requests-response *exchanges*, identified by a (small) unsigned integer. The gateway maintains a set of which exchange identifiers are free and may use them in any order. A gateway may open as many connections as it needs, but should strive to keep as few as possible.

A RAP *exchange* maintains the state of a request-response sequence or WebSocket connection. It also handles the per-exchange flow control mechanism, which is a simple transmission window with ACKs from the receiver. Exchanges inject *frames* into the connection for transmission.

A RAP *frame* is the basic structure within a stream. It consists of a *frame header* followed by the *frame body* data bytes.

A RAP *frame header* is 32 bits, divided into a 16-bit Size value, a 3-bit control field and a 13-bit exchange Index. If Index is 0x1fff (highest possible), the frame is a stream control frame and the control field is a 3-bit MSB value specifying the frame type:
* 000 - Ping, Size is a number to return in a Pong
* 001 - Setup, set up string mapping table, Size is bytes of data
* 010 - Stopping, no new exchanges, Size is bytes of optional UTF-8 message
* 011 - Stopped, conn closing now, Size is bytes of optional UTF-8 message
* 100 - Pong, Size is the value received in the Ping
* 101 - reserved
* 110 - reserved
* 111 - reserved

If Index is 0..0x1ffe (inclusive), the frame applies to that exchange, and the control field is mapped to three flags: Final, Head and Body. If neither Head nor Body flags are set, the frame is a flow control frame and the Size is ignored.
* Final - if set, this is the final frame for the exchange
* Head - if set, the data bytes starts with a RAP *record*
* Body - if set, data bytes form body data (after any RAP record, if present)

## RAP records

A RAP *record* type defines how the data bytes are encoded. The records have fields that are encoded using *RAP data types* such as *string* or *uint16*. Their definitions can be found in the section *RAP data types*.

### Setup record

Set up the string lookup table for a stream. May be sent once (and only once) from the server immediately after accepting a connection from a gateway and before any other records are sent. Once received by the gateway, the gateway may use the string lookup table provided in further frames.
* One or more of:
 * `byte` Lookup index. Must be a value between 2 and 255, inclusive.
 * `string` Lookup string.
* `0x00` Terminator. Signals the end of the table.

### HTTP request record

Sent from the gateway to start a new HTTP exchange. The record structure contains enough information to transparently carry a HTTP/1.1 request. Since the gateway must validate incoming requests and format them into request records, the upstream server receiving them may rely on the structure being correct.
* `string` HTTP method, e.g. `GET`.
* `string` URI path component using `/` as a separator. It must be URI-decoded (no `%xx`). Must be absolute (start with a `/`) and normalized, meaning it must not contain any `.` or `..` elements.
* `kvv` URI query component. Both keys and values must be URI-encoded. An empty `kvv` implies no query portion was present. This means the protocol cannot distinguish `/some/path` from `/some/path?`.
* `kvv` HTTP request headers. Keys must be in `Canonical-Format`. Values must comply with RFC 2616 section 4.2. Note that the `Host` and `Content-Length` headers are provided separately at the end, and must not appear here.
* `string` HTTP `Host` header value.
* `int64` HTTP `Content-Length` header value. If `-1`, then `Content-Length` header is not present.

### HTTP response record

Sent from the upstream server in response to a HTTP request record.
* `uint16` HTTP status code. Must be in the range 100-599, inclusive.
* `kvv` HTTP response headers. Keys must be in `Canonical-Format`. Values must comply with RFC 2616 section 4.2. The gateway must supply any required headers that are omitted, so that upstream need not send `Date` or `Server`.
* `string` HTTP `Status` header value. If the `Status` HTTP header is not present, and this value is not a NULL string, the gateway will insert a `Status` header with the value given.
* `int64` HTTP `Content-Length` header value. If the `Content-Length` HTTP header is not present, and this value is not negative, the gateway will insert a `Content-Length` header with the value given.

## RAP data types

### `uint64`

MSB encoded 7 bits at a time, with the high bit functioning as a stop bit. For each byte, the lower 7 bits are appended to the result. If the high bit is set, keep going. If the high bit is clear, return the result.

### `int64`

If the value is zero or positive, shift the value left one bit and encode as a `uint64`.
If the value is negative, shift the absolute value left one bit and set the lowest bit to one, then encode as a `uint64`.

### `uint16`

MSB encoded. Used for encoding HTTP status codes.

### `length`

Used to encode non-negative small integers primarily for string lengths.
A negative value or a value greater than 32767 cannot be encoded and is an error.
If the value is less than 128, write it as a single byte.
Otherwise, set bit 15 and encode it using `uint16`.

### `string`

A string encoding starts with a `length`. If the length is nonzero, then that many binary bytes follow.
A zero length signals special case handling and is followed by a single byte, interpreted as follows:
* `0x00` Null string. Used to mark the end of a list of strings.
* `0x01` Empty string, i.e. `""`.
* Other values refer to entries in the stream string lookup table. If an undefined string lookup value is seen, it is a fatal error and the stream must be closed.

### `kvv`

A key-value-value structure used to encode request query parameters and request-response headers.
* Zero or more of: `string` Key. Each key has zero or more values.
 * Zero or more of: `string` Value. Unordered value associated with the key.
 * `string` Null string (`0x00 0x00`) marking the end of values for the key.
* `string` Null string (`0x00 0x00`) marking the end of keys for this `kvv` set.

## Flow control

Each side of an exchange maintains a count of non-final data frames sent but not acknowledged. Frames sent with the exchange id set to 0x1fff, those with the final bit set, or those without payload are not counted.

A receiver must be able to buffer the full window size count of frames per exchange. When a received frame that is counted is processed, the receiver must acknowledge receipt of it by sending a frame header with the same exchange id, control bits set to `000` (not final, no head data, no body data) and the size value set to zero. This is called a *flow control frame*.

Before an exchange is done and it's id may be reused, both sides must send and receive a frame with the final control bit set. After the final frame is sent, only flow control frames may be sent. Upon receiving a final frame, we must either send a final frame in response if we haven't already, or release the exchange for reuse. After sending a final frame, we must wait for a final frame in response if we haven't already got one, and then wait for the flow control window to drain.

## License

The RAP specification is Copyright :copyright: 2015 Starcounter AB

Questions can be directed to the author, [Johan Lindh](https://github.com/linkdata)
