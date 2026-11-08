"""Optional offline smoke test for the actual core binary; never touches a router."""
import os
import pathlib
import socket
import subprocess
import time
import unittest


def reserve_ports(count):
    sockets = []
    try:
        for _ in range(count):
            sock = socket.socket()
            sock.bind(("127.0.0.1", 0))
            sockets.append(sock)
        return [sock.getsockname()[1] for sock in sockets]
    finally:
        for sock in sockets:
            sock.close()


class ClientSmokeTest(unittest.TestCase):
    @unittest.skipUnless(os.environ.get("STARLIGHT_TEST_BINARY"), "set STARLIGHT_TEST_BINARY to a native client binary")
    def test_local_listeners_do_not_imply_ground_readiness(self):
        binary = pathlib.Path(os.environ["STARLIGHT_TEST_BINARY"]).resolve()
        ground_socket = socket.socket()
        ground_socket.bind(("127.0.0.1", 0))  # Reserve but do not listen.
        self.addCleanup(ground_socket.close)
        ground = ground_socket.getsockname()[1]
        socks, http, fast = reserve_ports(3)
        env = dict(os.environ, SATMUX_PSK="abcd")
        process = subprocess.Popen([str(binary), "client", "--host", "127.0.0.1", "--port", str(ground),
                                    "--socks-port", str(socks), "--http-port", str(http), "--fast-port", str(fast)],
                                   env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            deadline = time.monotonic() + 5
            while time.monotonic() < deadline:
                self.assertIsNone(process.poll(), "client exited before its listeners were ready")
                try:
                    with socket.create_connection(("127.0.0.1", socks), timeout=0.2):
                        break
                except OSError:
                    time.sleep(0.05)
            else:
                self.fail("local SOCKS listener did not start")
            # No server was started at the ground port, yet the client runs.
            self.assertIsNone(process.poll())
        finally:
            process.terminate()
            try:
                process.communicate(timeout=5)
            except subprocess.TimeoutExpired:
                process.kill()
                process.communicate()
                self.fail("client did not stop after SIGTERM")


if __name__ == "__main__":
    unittest.main()
