from __future__ import absolute_import
import logging
import random
import socket
import time

import cbor


logger = logging.getLogger(__name__)


class SocketReader(object):
    '''
    Simple adapter from socket.recv to file-like-read
    '''
    def __init__(self, sock):
        self.socket = sock
        self.timeout_seconds = 10.0

    def read(self, num):
        start = time.time()
        data = self.socket.recv(num)
        while len(data) < num:
            now = time.time()
            if now > (start + self.timeout_seconds):
                break
            ndat = self.socket.recv(num - len(data))
            if ndat:
                data += ndat
        return data


class CborRpcClient(object):
    '''Base class for all client objects.

    This provides common `addr_family`, `address`, and `registry_addresses`
    configuration parameters, and manages the connection back to the server.

    Automatic retry and time based fallback is managed from
    configuration parameters `retries` (default 5), and
    `base_retry_seconds` (default 0.5). Retry time doubles on each
    retry. E.g. try 0; wait 0.5s; try 1; wait 1s; try 2; wait 2s; try
    3; wait 4s; try 4; wait 8s; try 5; FAIL. Total time waited just
    under base_retry_seconds * (2 ** retries).

    .. automethod:: __init__
    .. automethod:: _rpc
    .. automethod:: close

    '''

    def __init__(self, config=None):
        self._socket_family = config.get('addr_family', socket.AF_INET)
        # may need to be ('host', port)
        self._socket_addr = config.get('address')
        if self._socket_family == socket.AF_INET:
            if not isinstance(self._socket_addr, tuple):
                # python socket standard library insists this be tuple!
                tsocket_addr = tuple(self._socket_addr)
                assert len(tsocket_addr) == 2, 'address must be length-2 tuple ("hostname", port number), got {!r} tuplified to {!r}'.format(self._socket_addr, tsocket_addr)
                self._socket_addr = tsocket_addr
        self._socket = None
        self._rfile = None
        self._local_addr = None
        self._message_count = 0
        self._retries = config.get('retries', 5)
        self._base_retry_seconds = float(config.get('base_retry_seconds', 0.5))

    def _conn(self):
        # lazy socket opener
        if self._socket is None:
            try:
                self._socket = socket.create_connection(self._socket_addr)
                self._local_addr = self._socket.getsockname()
            except:
                logger.error('error connecting to %r:%r', self._socket_addr[0],
                             self._socket_addr[1], exc_info=True)
                raise
        return self._socket

    def close(self):
        '''Close the connection to the server.

        The next RPC call will reopen the connection.

        '''
        if self._socket is not None:
            self._rfile = None
            try:
                self._socket.shutdown(socket.SHUT_RDWR)
                self._socket.close()
            except socket.error:
                logger.warn('error closing lockd client socket',
                            exc_info=True)
            self._socket = None

    @property
    def rfile(self):
        if self._rfile is None:
            conn = self._conn()
            self._rfile = SocketReader(conn)
        return self._rfile

    def _rpc(self, method_name, params):
        '''Call a method on the server.

        Calls ``method_name(*params)`` remotely, and returns the results
        of that function call.  Expected return types are primitives, lists,
        and dictionaries.

        :raise Exception: if the server response was a failure

        '''
        mlog = logging.getLogger('cborrpc')
        tryn = 0
        delay = self._base_retry_seconds
        self._message_count += 1
        message = {
            'id': self._message_count,
            'method': method_name,
            'params': params
        }
        mlog.debug('request %r', message)
        buf = cbor.dumps(message)

        errormessage = None
        while True:
            try:
                conn = self._conn()
                conn.send(buf)
                response = cbor.load(self.rfile)
                mlog.debug('response %r', response)
                assert response['id'] == message['id']
                if 'result' in response:
                    return response['result']
                # From here on out we got a response, the server
                # didn't have some weird intermittent error or
                # non-connectivity, it gave us an error message. We
                # don't retry that, we raise it to the user.
                errormessage = response.get('error')
                if errormessage and hasattr(errormessage,'get'):
                    errormessage = errormessage.get('message')
                if not errormessage:
                    errormessage = repr(response)
                break
            except Exception as ex:
                if tryn < self._retries:
                    tryn += 1
                    logger.debug('ex in %r (%s), retrying %s in %s sec...',
                                 method_name, ex, tryn, delay, exc_info=True)
                    self.close()
                    time.sleep(delay)
                    delay *= 2
                    continue
                logger.error('failed in rpc %r %r', method_name, params,
                             exc_info=True)
                raise
        raise Exception(errormessage)


if __name__ == '__main__':
    import sys
    logging.basicConfig(level=logging.DEBUG)
    host,port = sys.argv[1].split(':')
    if not host:
        host = 'localhost'
    port = int(port)
    client = CborRpcClient({'address':(host,port)})
    print(client._rpc(u'connect', [u'127.0.0.1:5432', u'root', u'aoeu']))
    print(client._rpc(u'put', [[('k1','v1'), ('k2','v2')]]))
    #print(client._rpc(u'ping', []))
    #print(client._rpc(u'gnip', []))
    client.close()
        
