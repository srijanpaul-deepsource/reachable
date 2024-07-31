def iszero(x):
    return x == 0


def assert_nonzero(x):
    if iszero(x):
        print("OH NO!")
        raise Exception("Division by zero")
