import unittest

from tag import valid_tag


class TagTests(unittest.TestCase):
    def test_valid_tags(self):
        for tag in ("v0.0.0", "v1.2.3", "v1.2.3-rc.1", "v1.2.3+001", "v1.2.3-01a+build.42"):
            with self.subTest(tag=tag):
                self.assertTrue(valid_tag(tag))

    def test_invalid_tags(self):
        for tag in ("1.2.3", "v01.2.3", "v1.02.3", "v1.2.03", "v1.2.3-01", "v1.2.3-", "v1.2.3+", "v1.2", "v1.2.3/extra"):
            with self.subTest(tag=tag):
                self.assertFalse(valid_tag(tag))


if __name__ == "__main__":
    unittest.main()
