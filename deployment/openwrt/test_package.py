import importlib.util
import io
import pathlib
import struct
import subprocess
import tarfile
import tempfile
import unittest

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("build_ipk", HERE / "build_ipk.py")
builder = importlib.util.module_from_spec(spec)
spec.loader.exec_module(builder)


def members(data):
    with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as tar:
        return {item.name: (item, tar.extractfile(item).read() if item.isfile() else None) for item in tar.getmembers()}


def fake_elf():
    data = bytearray(120)
    data[:7] = b"\x7fELF\x02\x01\x01"
    data[16:18] = (2).to_bytes(2, "little")
    data[18:20] = (183).to_bytes(2, "little")
    struct.pack_into("<I", data, 20, 1)
    struct.pack_into("<Q", data, 32, 64)
    struct.pack_into("<HHH", data, 52, 64, 56, 1)
    struct.pack_into("<IIQQQQQQ", data, 64, 1, 5, 0, 0, 0, len(data), len(data), 0)
    return bytes(data)


class PackageTests(unittest.TestCase):
    def test_layout_and_reproducibility(self):
        ipk = builder.build(fake_elf(), "0.1.0-1")
        self.assertEqual(ipk, builder.build(fake_elf(), "0.1.0-1"))
        outer = members(ipk)
        self.assertEqual(outer["./debian-binary"][1], b"2.0\n")
        control = members(outer["./control.tar.gz"][1])
        self.assertIn(b"Architecture: aarch64_cortex-a53\n", control["./control"][1])
        self.assertIn(b"/etc/starlight/client.conf", control["./conffiles"][1])
        data = members(outer["./data.tar.gz"][1])
        self.assertEqual(data["./etc/starlight/client.conf"][0].mode, 0o600)
        self.assertTrue(data["./etc/starlight"][0].isdir())
        self.assertEqual(data["./etc/starlight"][0].mode, 0o700)
        self.assertEqual(data["./etc/init.d/starlight"][0].mode, 0o755)
        self.assertEqual(data["./usr/sbin/starlight"][1], fake_elf())
        self.assertIn("./usr/share/oui/menu.d/starlight.json", data)
        self.assertIn("./www/views/gl-sdk4-ui-starlight.common.js", data)
        self.assertIn("./usr/lib/oui-httpd/rpc/starlight", data)
        self.assertEqual(set(control), {"./control", "./conffiles"})

    def test_reject_invalid_inputs(self):
        for binary, version in [(b"bad", "1"), (fake_elf(), "1\nInjected: yes"), (fake_elf(), "../1")]:
            with self.subTest(version=version), self.assertRaises(ValueError):
                builder.build(binary, version)

    def test_reject_malformed_or_dynamic_elf(self):
        valid = fake_elf()
        dynamic = bytearray(valid)
        struct.pack_into("<I", dynamic, 64, 3)
        truncated_header = bytearray(valid)
        struct.pack_into("<H", truncated_header, 56, 2)
        truncated_segment = bytearray(valid)
        struct.pack_into("<Q", truncated_segment, 96, len(valid) + 1)
        wrong_arch = bytearray(valid)
        struct.pack_into("<H", wrong_arch, 18, 62)
        for binary in (valid[:20], dynamic, truncated_header, truncated_segment, wrong_arch):
            with self.subTest(binary=bytes(binary)[:24]), self.assertRaises(ValueError):
                builder.build(bytes(binary), "1")


class ServiceTests(unittest.TestCase):
    def run_service(self, content, owner="0", mode="600"):
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "client.conf"
            path.write_text(content)
            init = (HERE / "starlight.init").read_text().replace("/etc/starlight/client.conf", str(path))
            script = pathlib.Path(directory) / "init"
            script.write_text(init)
            harness = f'''
stat() {{ case "$2" in %u) printf '%s' '{owner}' ;; %a) printf '%s' '{mode}' ;; esac; }}
logger() {{ :; }}
procd_open_instance() {{ echo open; }}
procd_set_param() {{ case "$1" in env) echo env-set ;; *) printf '%s\\n' "$*" ;; esac; }}
procd_close_instance() {{ echo close; }}
. "$1"
start_service
'''
            return subprocess.run(["sh", "-c", harness, "test", str(script)], text=True, capture_output=True)

    def test_valid_configuration(self):
        result = self.run_service("host=ground.example\nport=8443\npsk=abcdef12\n")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("command /usr/sbin/starlight client --host ground.example --port 8443", result.stdout)
        self.assertIn("respawn 3600 5 5", result.stdout)
        self.assertNotIn("abcdef12", result.stdout + result.stderr)

    def test_custom_resource_and_port_configuration(self):
        result = self.run_service("host=ground.example\nport=8443\npsk=abcdef12\n"
                                  "carriers=3\nsocks_port=11080\nhttp_port=18080\nfast_port=18081\n")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("--carriers 3 --socks-port 11080 --http-port 18080 --fast-port 18081", result.stdout)

    def test_experimental_transparent_port(self):
        base = "host=ground.example\nport=8443\npsk=abcdef12\n"
        result = self.run_service(base + "transparent_port=19090\n")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("--transparent-port 19090", result.stdout)
        for value in ("8080", "65536", "01", "abc"):
            with self.subTest(value=value):
                self.assertNotEqual(self.run_service(base + "transparent_port=" + value + "\n").returncode, 0)
        self.assertNotEqual(self.run_service(base + "transparent_port=19090\ntransparent_port=19091\n").returncode, 0)

    def test_invalid_configuration_does_not_start(self):
        cases = ["host=\nport=8443\npsk=aa\n", "host=x\nport=0\npsk=aa\n",
                 "host=x\nport=8443\npsk=abc\n", "host=x\nport=8443\npsk=aa\nhost=y\n",
                 "host=x\nport=8443\npsk=aa\nother=x\n",
                 "host=x\nport=08443\npsk=aa\n", "host=x\nport=8443\npsk=aa\ncarriers=65\n",
                 "host=x\nport=8443\npsk=aa\nsocks_port=8080\n",
                 "host=x\nport=8443\npsk=aa\nfast_port=65536\n",
                 "host=x\nport=8443\npsk=aa\ncarriers=1\ncarriers=2\n"]
        for config in cases:
            with self.subTest(config=config):
                result = self.run_service(config)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn("open", result.stdout)
        for owner, mode in [("1", "600"), ("0", "644")]:
            result = self.run_service("host=x\nport=8443\npsk=aa\n", owner, mode)
            self.assertNotEqual(result.returncode, 0)
            self.assertNotIn("open", result.stdout)


if __name__ == "__main__":
    unittest.main()
