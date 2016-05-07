
IPC
===

All communication is done over stdin/stdout. Currently there are no plans to support named pipes
or networks but such a feature is trivial to add so it can be added when needed.

The protocol is line-oriented and all requests are asynchronous.

Requests are a pair of JSON objects encoded as follows:

	{"token":"...", "method": "..."}{...}

The first object specifies what method to call and an optional token. If `method` is omitted,
the request is ignored. `token` is an optional value the client may use to identify responses.
The second object is defined by the method.

Responses are returned as a JSON object encoded as follows:

	{"token": "...", "error": "...", "data": {...}}

`token` is the original `token` passed in the corresponding request (if any).
`error` desribes any possible error that occurred while handling the request.
`data` is defined by the method.


Methods
=======

**hello** `{"s": "..."}` -> `{"s": "..."}`

	hello takes an object with a key `s` and returns it

**ping** `{"delay": 0}` -> `{"start": "...", "end": "..."}`

	ping takes an object with an optional `delay` in `milliseconds` and returns an object specifying
	the `start` time when the request was received and `end` specifying when the delay ended.
