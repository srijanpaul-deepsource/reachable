from mypackage.util import assert_nonzero


def div(a, b):
    assert_nonzero(b)
    return a / b


def main():
    a = int(input("a: "))
    b = int(input("b: "))
    res = div(a, b)
    print(res)


if __name__ == "__main__":
    main()
