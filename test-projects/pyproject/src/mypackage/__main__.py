from mypackage.util import assert_nonzero
from requests import get

def div(a, b):
    assert_nonzero(b)
    assert_nonzero(b)
    return a / b


def main():
    a = int(input("a: "))
    b = int(input("b: "))
    res = div(a, b)
    print(res)

def request_from_dog_ceo():
    response = get("https://dog.ceo/api/breeds/image/random")
    print(response.json())

if __name__ == "__main__":
    main()
