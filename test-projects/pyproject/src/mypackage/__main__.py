from mypackage.util import assert_nonzero
from requests import get
from fastapi.routing import APIRoute


def div(a, b):
    assert_nonzero(b)
    assert_nonzero(b)
    return a / b


def main():
    a = int(input("a: "))
    b = int(input("b: "))
    res = div(a, b)
    print(res)
    request_from_dog_ceo()


def request_from_dog_ceo():
    response = get("https://dog.ceo/api/breeds/image/random")
    print(response.json())

    APIRoute("/foo")


if __name__ == "__main__":
    main()
