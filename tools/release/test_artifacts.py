import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import artifacts


class ArtifactTests(unittest.TestCase):
    def test_create_verify_and_detect_tampering(self):
        with tempfile.TemporaryDirectory() as tmp:
            directory = Path(tmp)
            for name in artifacts.names("v1.2.3"):
                (directory / name).write_bytes(name.encode())
            script = Path(artifacts.__file__)
            command = [sys.executable, str(script)]
            subprocess.run(command + ["create", tmp, "v1.2.3"], check=True)
            subprocess.run(command + ["verify", tmp, "v1.2.3"], check=True)
            (directory / artifacts.names("v1.2.3")[0]).write_bytes(b"modified")
            self.assertNotEqual(subprocess.run(command + ["verify", tmp, "v1.2.3"], capture_output=True).returncode, 0)

    def test_missing_and_invalid_artifacts(self):
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaises(ValueError):
                artifacts.names("../unsafe")
            with self.assertRaises(ValueError):
                artifacts.manifest(Path(tmp), "v1.2.3")

    def test_manifest_requires_exact_entries(self):
        with tempfile.TemporaryDirectory() as tmp:
            directory = Path(tmp)
            for name in artifacts.names("v1.2.3"):
                (directory / name).write_bytes(b"binary")
            (directory / "checksums.txt").write_text(artifacts.manifest(directory, "v1.2.3") + "extra\n")
            script = Path(artifacts.__file__)
            result = subprocess.run([sys.executable, str(script), "verify", tmp, "v1.2.3"], capture_output=True)
            self.assertNotEqual(result.returncode, 0)


if __name__ == "__main__":
    unittest.main()
