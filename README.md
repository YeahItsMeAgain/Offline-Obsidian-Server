# About
Offline obsidian server (Plugins, Themes, Automatic Updates).
![example](./example.png)

# Setup
- Run the [downloader](./downloader/main.go).
- Patch the releases with [patcher.py](./patcher/patcher.py), use --patch_releases.
- Setup nginx with the [config](./nginx/nginx.conf), make sure the paths are correct.
- To patch clients to use the [patcher.py](./patcher/patcher.py) with the server address as an argument.

# Update
To copy only the new files after an update you can use the following commands:
```bash
cd downloader
touch /tmp/new
go run main.go
mkdir new
find files/ -newer /tmp/new -exec cp --parents \{\} ./new \; 
```

# Notes
- This probably breaks stuff in the obsidian app.
- Tested on the following obsidian versions: v1.0.3, v1.1.9 