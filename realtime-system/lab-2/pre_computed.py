def calculate_period(min_indicate: int, max_indicate: int, base_frequency: int) -> list[int]:
    period_map = []
    period = range(-10, 14, 1)
    for i in period:
        period_map.append(int(1000000/(2*(440*2**(i/12)))))

    return period_map

if __name__ == "__main__":
    min_indicate = int(input("The minimum indicate: "))
    max_indicate = int(input("The maximum indicate: "))
    base_frequency = int(input("The base value of frequency: "))

    ret = calculate_period(min_indicate, max_indicate, base_frequency)

    print(ret)
    