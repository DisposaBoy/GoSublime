#!/bin/sh -x

python -m cbor.tests.test_cbor
python -m cbor.tests.test_objects
python -m cbor.tests.test_usage
python -m cbor.tests.test_vectors

#python cbor/tests/test_cbor.py
#python cbor/tests/test_objects.py
#python cbor/tests/test_usage.py
#python cbor/tests/test_vectors.py
