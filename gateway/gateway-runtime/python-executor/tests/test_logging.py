import io
import logging
import unittest

from main import (
    ComponentPrefixFormatter,
    LOG_COMPONENT_PREFIX,
    setup_logging,
)


class ComponentPrefixFormatterTest(unittest.TestCase):
    """The container multiplexes Envoy, the policy engine and this process onto
    one stdout, so every emitted line has to carry the component tag."""

    def _emit(self, fn):
        stream = io.StringIO()
        handler = logging.StreamHandler(stream)
        handler.setFormatter(ComponentPrefixFormatter(
            fmt='%(asctime)s [%(levelname)s] %(name)s: %(message)s',
            datefmt='%Y-%m-%d %H:%M:%S'))
        logger = logging.getLogger('test.' + fn.__name__)
        logger.handlers = [handler]
        logger.setLevel(logging.INFO)
        logger.propagate = False
        fn(logger)
        handler.flush()
        return [ln for ln in stream.getvalue().split('\n') if ln]

    def test_single_line_is_tagged(self):
        lines = self._emit(lambda log: log.info('started'))
        self.assertEqual(1, len(lines))
        self.assertTrue(lines[0].startswith(LOG_COMPONENT_PREFIX))

    def test_every_traceback_line_is_tagged(self):
        def emit(log):
            try:
                raise ValueError('boom')
            except ValueError:
                log.exception('policy execution failed')

        lines = self._emit(emit)
        # A traceback spans several lines; the point of the formatter is that the
        # tag is not limited to the first.
        self.assertGreater(len(lines), 2)
        for line in lines:
            self.assertTrue(line.startswith(LOG_COMPONENT_PREFIX), line)

    def test_every_line_of_a_multiline_message_is_tagged(self):
        lines = self._emit(lambda log: log.info('one\ntwo\nthree'))
        self.assertEqual(3, len(lines))
        for line in lines:
            self.assertTrue(line.startswith(LOG_COMPONENT_PREFIX), line)

    def test_tag_is_not_applied_twice(self):
        lines = self._emit(lambda log: log.info('started'))
        self.assertFalse(
            lines[0].startswith(LOG_COMPONENT_PREFIX + LOG_COMPONENT_PREFIX))


class SetupLoggingWiringTest(unittest.TestCase):
    """The tests above exercise the formatter directly; these pin the wiring, so
    that reverting to a plain Formatter or moving the tag back into the format
    string is caught rather than only the class being covered."""

    def setUp(self):
        root = logging.getLogger()
        self._saved = (root.handlers[:], root.level)

    def tearDown(self):
        root = logging.getLogger()
        root.handlers, root.level = self._saved

    def test_root_handler_uses_the_prefixing_formatter(self):
        logging.getLogger().handlers = []
        setup_logging()

        formatters = [h.formatter for h in logging.getLogger().handlers]
        self.assertTrue(
            any(isinstance(f, ComponentPrefixFormatter) for f in formatters))

    def test_format_string_does_not_also_carry_the_tag(self):
        # Both places applying the tag would emit it twice on the first line.
        logging.getLogger().handlers = []
        setup_logging()

        for handler in logging.getLogger().handlers:
            fmt = handler.formatter._style._fmt
            self.assertNotIn(LOG_COMPONENT_PREFIX.strip(), fmt)


if __name__ == '__main__':
    unittest.main()
