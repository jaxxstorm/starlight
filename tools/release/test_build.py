import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


class BuildTests(unittest.TestCase):
    def run_build(self, fail_target=None):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            fake_go = root / "go"
            fake_go.write_text(
                "#!/usr/bin/env python3\n"
                "import os, pathlib, sys\n"
                "assert os.environ['CGO_ENABLED'] == '0'\n"
                "arch = os.environ.get('GOARCH')\n"
                "if arch == os.environ.get('FAIL_TARGET'): sys.exit(1)\n"
                "pathlib.Path(sys.argv[sys.argv.index('-o') + 1]).write_text("
                "os.environ.get('GOOS', 'native') + '/' + str(arch) + '/' + os.environ.get('GOARM', '') + '/' + os.environ.get('GOARM64', '') + '/' + os.environ.get('GOAMD64', ''))\n"
            )
            fake_go.chmod(0o755)
            env = os.environ.copy()
            env["PATH"] = str(root) + os.pathsep + env["PATH"]
            env["GOARM"] = "6"
            env["GOARM64"] = "v8.8"
            env["GOAMD64"] = "v4"
            if fail_target:
                env["FAIL_TARGET"] = fail_target
            output = root / "dist"
            script = Path(__file__).with_name("build.py")
            result = subprocess.run([sys.executable, str(script), "matrix", "--output", str(output)], env=env, capture_output=True)
            return result.returncode, {p.name: p.read_text() for p in output.iterdir()}

    def test_matrix_settings(self):
        code, files = self.run_build()
        self.assertEqual(code, 0)
        self.assertEqual(files, {
            "starlight_dev_linux_amd64": "linux/amd64///v1",
            "starlight_dev_linux_arm64": "linux/arm64//v8.0/",
            "starlight_dev_linux_armv7": "linux/arm/7//",
        })

    def test_failed_build_produces_no_matrix(self):
        code, files = self.run_build("arm64")
        self.assertNotEqual(code, 0)
        self.assertEqual(files, {})


if __name__ == "__main__":
    unittest.main()
