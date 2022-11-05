# About
Offline obsidian plugins server.
![example](./example.png)

# Setup
- Run the [downloader](./downloader/main.go), use `--help` to view all the available options.
- Setup nginx with the [config](./nginx/nginx.conf), make sure the paths are correct.
- To patch clients to use the [patcher.py](./patcher/patcher.py) with the server address as an argument.

# Plugins Update
To copy only the new files after a plugins update you can use the following commands:
```bash
cd downloader
touch /tmp/new
go run main.go [-minimal?]
mkdir new-plugins
find plugins[-minimal?]/ -newer /tmp/new -exec cp --parents \{\} ./new-plugins \; 
```

# Notes
- This probably breaks stuff in the obsidian app.
- Tested on the following obsidian versions: v1.0.3