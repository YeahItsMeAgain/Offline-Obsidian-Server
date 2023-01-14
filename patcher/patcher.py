import argparse
import glob
import gzip
import os
import platform
import ctypes
import logging
import shutil
import subprocess
import time
from typing import Optional
import psutil
logging.basicConfig(level=logging.INFO)


ORIGINAL_SERVER_ADDRESSES = [
    "https://raw.githubusercontent.com",
    "https://github.com",
    "https://releases.obsidian.md"
]
SIGNATURE_VERIFICATION_METHODS = [
    "let verifiedHash =",
    "let verifiedSignature =",
]


def validate_rasar_is_available():
    try:
        subprocess.check_call("rasar  --version".split(), stdout=subprocess.DEVNULL)
    except subprocess.CalledProcessError:
        logging.fatal("[!] install rasar, https://github.com/Zerthox/rasar")
        exit(-1)


def validate_running_as_admin():
    if platform.system() == "Windows":
        if not ctypes.windll.shell32.IsUserAnAdmin():
            logging.fatal("[!] Run as admin!")
            exit(-1)
    elif os.getuid() != 0:
        logging.fatal("[!] Run as admin!")
        exit(-1)


def stop_running_obsidian():
    logging.info("[*] Stopping obsidian")
    for proc in psutil.process_iter():
        if proc.name().lower().startswith("obsidian"):
            proc.kill()
            time.sleep(3)


def unpack_asar(asar_path: str, asar_backup_path: Optional[str], extracted_asar_path: str):
    if asar_backup_path:
        if os.path.exists(asar_backup_path):
            logging.info("[*] Restoring original %s from backup", os.path.basename(asar_path))
            shutil.copy(asar_backup_path, asar_path)
        else:
            logging.info("[*] Backing up original %s", os.path.basename(asar_path))
            shutil.copy(asar_path, asar_backup_path)

    logging.info("[*] Extracting %s", os.path.basename(asar_path))
    subprocess.call(f"rasar e {asar_path} {extracted_asar_path}".split(
    ), stdout=subprocess.DEVNULL)

    if not os.path.exists(os.path.join(extracted_asar_path, 'app.js')) and \
            not os.path.exists(os.path.join(extracted_asar_path, 'main.js')):
        logging.fatal("[*] Cannot find app.js/main.js in %s", extracted_asar_path)
        exit(-1)


def patch_asar_folder(extracted_asar_path: str, offline_server: str):
    for js_file in [
        os.path.join(extracted_asar_path, 'app.js'),
        os.path.join(extracted_asar_path, 'main.js')
    ]:
        if os.path.exists(js_file):
            logging.info("[*] Replacing server in %s", js_file)
            with open(js_file, 'r', encoding='utf8') as file:
                filedata = file.read()
                for server in ORIGINAL_SERVER_ADDRESSES:
                    filedata = filedata.replace(server, offline_server)

                # let verifiedHash = hashComparison -> let verifiedHash = true || hashComparison;
                for signature_verification in SIGNATURE_VERIFICATION_METHODS:
                    filedata = filedata.replace(
                        signature_verification, f'{signature_verification} true || ')

                with open(js_file, 'w', encoding='utf8') as file:
                    file.write(filedata)


def pack_asar(asar_path: str, extracted_asar_path: str):
    logging.info("[*] Deleting old %s", os.path.basename(asar_path))
    os.unlink(asar_path)

    logging.info("[*] Packing new %s", os.path.basename(asar_path))
    subprocess.call(f"rasar p {extracted_asar_path} {asar_path}".split(), stdout=subprocess.DEVNULL)

    logging.info("[*] Deleting extracted %s", os.path.basename(asar_path))
    shutil.rmtree(extracted_asar_path)


def patch_asar(asar_path: str, server_address: str, backup_asar: bool = False):
    asar_backup_path = f'{asar_path}.bak' if backup_asar else None
    extracted_asar_path = f'{asar_path}.extracted'

    unpack_asar(asar_path, asar_backup_path, extracted_asar_path)
    patch_asar_folder(extracted_asar_path, server_address)
    pack_asar(asar_path, extracted_asar_path)


def patch_obsidian_releases(offline_server: str):
    desktop_releases_path = os.path.join(os.path.dirname(
        __file__), '../downloader/files/obsidianmd/obsidian-releases/desktop-releases.json')
    logging.info("[*] Switching server in %s", desktop_releases_path)
    with open(desktop_releases_path, 'r', encoding='utf8') as desktop_releases:
        filedata = desktop_releases.read()
        for server in ORIGINAL_SERVER_ADDRESSES:
            filedata = filedata.replace(server, offline_server)

    with open(desktop_releases_path, 'w', encoding='utf-8') as desktop_releases:
        desktop_releases.write(filedata)

    for gzipped_release_path in glob.iglob(
        os.path.join(os.path.dirname(
            __file__), '../downloader/files/obsidianmd/obsidian-releases/releases/download/*/*.gz'),
        recursive=True
    ):
        release_path = os.path.splitext(gzipped_release_path)[0]
        with gzip.open(gzipped_release_path, 'rb') as gzipped_release:
            with open(release_path, 'wb') as release:
                shutil.copyfileobj(gzipped_release, release)

        patch_asar(release_path, offline_server, False)

        with open(release_path, 'rb') as release:
            with gzip.open(gzipped_release_path, 'wb') as gzipped_release:
                shutil.copyfileobj(release, gzipped_release)

        os.unlink(release_path)

def delete_obsidian_cache():
    for obsidian_asar_cache in glob.glob(os.path.join(os.getenv("APPDATA"), 'obsidian', '*.asar')):
        logging.info('[*] Removing cached asar -> %s', obsidian_asar_cache)
        os.remove(obsidian_asar_cache)

def main():
    validate_rasar_is_available()

    parser = argparse.ArgumentParser()
    parser.add_argument("--server", help="Server address",
                        default="http://obsidian-server/files")
    parser.add_argument("--patch_releases",
                        help="Patch every release file", action="store_true")
    args = parser.parse_args()
    logging.info('[*] Configured server: %s', args.server)

    if args.patch_releases:
        patch_obsidian_releases(args.server)
        return

    # Validating the environment.
    validate_running_as_admin()
    stop_running_obsidian()

    # TODO: Obsidian path in linux.
    obsidian_resources_folder = os.path.join(os.getenv("LOCALAPPDATA"), 'Obsidian', 'resources')
    if not os.path.exists(obsidian_resources_folder):
        logging.fatal("[!] Obsidian is not installed")
        exit(-1)

    delete_obsidian_cache()

    for asar_file in ['obsidian.asar', 'app.asar']:
        patch_asar(os.path.join(obsidian_resources_folder, asar_file), args.server, True)


if __name__ == '__main__':
    main()
