import sys
import pytest


def test_pass():
    print("synthetic stdout")
    print("synthetic stderr", file=sys.stderr)


def test_fail():
    assert 1 == 2, "synthetic failure"


@pytest.mark.skip(reason="synthetic skip")
def test_skip():
    pass


@pytest.fixture
def broken():
    raise RuntimeError("synthetic setup error")


def test_error(broken):
    pass


class TestOther:
    def test_pass(self):
        pass
