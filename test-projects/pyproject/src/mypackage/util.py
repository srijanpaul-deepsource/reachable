def iszero(x):
    return x == 0


def assert_nonzero(x):
    if iszero(x):
        raise Exception("Division by zero")
