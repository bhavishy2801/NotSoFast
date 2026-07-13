"""Scripted protocol check only, never counted as a model experiment."""
import json
from pathlib import Path
import sys
import tempfile
import unittest
from model_harness import trial


class HarnessContract(unittest.TestCase):
    def test_driver_contract(self):
        with tempfile.TemporaryDirectory() as temp:
            driver = Path(temp) / "driver.py"
            driver.write_text("import json,sys\nq=json.load(sys.stdin)\nprint(json.dumps({'message':{'role':'assistant','content':'No action'},'usage':None}))\n")
            result = trial([sys.executable, str(driver)], "E", "fresh.txt", {}, 1)
            self.assertFalse(result["published"])
            self.assertTrue(result["blocked_valid"])
            self.assertEqual(result["usage"], [None])


if __name__ == "__main__":
    unittest.main()
