import unittest

from sdk.types import Headers


class HeadersTest(unittest.TestCase):
    def test_headers_are_case_insensitive_and_defensive(self):
        headers = Headers(
            {
                "X-Test": ["one", "two"],
                "Content-Type": ["application/json"],
            }
        )

        values = headers.get("x-test")
        values.append("three")

        all_headers = headers.get_all()
        all_headers["x-test"].append("four")

        self.assertEqual(["one", "two"], headers.get("X-Test"))
        self.assertTrue(headers.has("content-type"))
        self.assertEqual(["application/json"], headers.get("CONTENT-TYPE"))
        self.assertEqual(
            {
                "x-test": ["one", "two"],
                "content-type": ["application/json"],
            },
            dict(headers.iterate()),
        )
        self.assertEqual(2, len(headers))
        self.assertCountEqual(["x-test", "content-type"], list(headers))
