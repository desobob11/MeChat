

files = ["server.go", "serverutils.go", "serverManager.go"]

for i in range(4):
    for f in files:
        with open(f"C:/Users/Des/MeChat/server/{f}", "rb") as src:
            data = src.read()
            with open(f"C:/Users/Des/replicas/r{i}/server/{f}", "wb") as dest:
                dest.write(data)

