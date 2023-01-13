import os
import sys
import platform
import ctypes
import logging
import shutil
import subprocess
import psutil
logging.basicConfig(level=logging.INFO)

ORIGINAL_SERVER_ADDRESSES = ["https://raw.githubusercontent.com", "https://github.com", "https://releases.obsidian.md"]
OFFLINE_SERVER_ADDRESS = sys.argv[1] if len(sys.argv) > 1 else "http://obsidian-server/files"
logging.info('[*] Configured server: %s', OFFLINE_SERVER_ADDRESS)

# Importing dependencies.
try:
    subprocess.check_call("rasar  --version".split(), stdout=subprocess.DEVNULL)
except subprocess.CalledProcessError:
    logging.fatal("[!] install rasar, https://github.com/Zerthox/rasar")
    exit(-1)


# Making sure that running as admin.
if platform.system() == "Windows":
    if not ctypes.windll.shell32.IsUserAnAdmin():
        logging.fatal("[!] Run as admin!")
        exit(-1)
else:
    if os.getuid() != 0:
        logging.fatal("[!] Run as admin!")
        exit(-1)


# Stopping obsidian.
logging.info("[*] Stopping obsidian")
for proc in psutil.process_iter():
    if proc.name().lower().startswith("obsidian"):
        proc.kill()


# TODO: obsidian path in linux.
# Making sure obsidian is installed.
asar_folder = os.path.join(
    os.getenv("LOCALAPPDATA"), 'Obsidian', 'resources')
asar_path = os.path.join(asar_folder, 'obsidian.asar')
asar_backup = os.path.join(asar_folder, 'obsidian.asar.bak')
extracted_asar_path = os.path.join(
    asar_folder, 'obsidian.asar.extracted')
if not os.path.exists(asar_folder) or not os.path.exists(asar_path):
    logging.fatal("[!] Obsidian is not installed")
    exit(-1)

if os.path.exists(asar_backup):
    logging.info("[*] Restoring original obsidian.asar from backup")
    shutil.copy(asar_backup, asar_path)
else:
    logging.info("[*] Backing up original obsidian.asar")
    shutil.copy(asar_path, asar_backup)


logging.info("[*] Extracting obsidian.asar -> obsidian.asar.extracted")
subprocess.call(f"rasar e {asar_path} {extracted_asar_path}".split(), stdout=subprocess.DEVNULL)

app_js_path = os.path.join(extracted_asar_path, 'app.js')
if not os.path.exists(app_js_path):
    logging.fatal("[*] Cannot find app.js")
    exit(-1)

logging.info("[*] Replacing server in app.js")
with open(app_js_path, 'r', encoding='utf8') as file:
    filedata = file.read()

for server in ORIGINAL_SERVER_ADDRESSES:
    filedata = filedata.replace(server, OFFLINE_SERVER_ADDRESS)


with open(app_js_path, 'w', encoding='utf8') as file:
    file.write(filedata)

logging.info("[*] Deleting old obsidian.asar")
os.unlink(asar_path)

logging.info("[*] Packing new obsidian.asar")
subprocess.call(f"rasar p {extracted_asar_path} {asar_path}".split(), stdout=subprocess.DEVNULL)

logging.info("[*] Deleting extracted obsidian.asar")
shutil.rmtree(extracted_asar_path)

logging.info("[*] Finished")
