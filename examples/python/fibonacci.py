# Fibonacci example for Zerverless
# INPUT should contain {"n": <number>}

import json

def fib(n):
    if n <= 1:
        return n
    a, b = 0, 1
    for _ in range(2, n + 1):
        a, b = b, a + b
    return b

n = INPUT.get("n", 10)
result = fib(n)

# Output as JSON
print(json.dumps({"fibonacci": result, "n": n}))


