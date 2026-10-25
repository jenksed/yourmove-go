import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('candidate', Path(__file__).with_name('native-candidate.py'))
candidate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(candidate)

class NativeIdentityTests(unittest.TestCase):
    def test_stale_or_unrelated_window_cannot_pass(self):
        expected = dict(run_id='new', workspace_marker='A', pid=42, executable='/candidate/YourMove', sha256='hash', working_directory='/A', data_root='/A', database='/A/jobs.db', resumes='/A/resumes', lifetime='today')
        record = dict(expected, window_title='YourMove · new · PID 42')
        owner = dict(pid=42, executable=expected['executable'], windows=[dict(owner_pid=42, title=record['window_title'], layer=0, onscreen=True, window_id=12)])
        self.assertEqual(candidate.check_identity(expected,record,owner,'today',True),12)
        for changed in [dict(owner,pid=41),dict(owner,executable='/installed/YourMove'),dict(owner,windows=[]),dict(owner,windows=[dict(owner_pid=41,title=record['window_title'],layer=0,onscreen=True,window_id=13)]),dict(owner,windows=[dict(owner_pid=42,title='YourMove',layer=0,onscreen=True,window_id=13)])]:
            with self.assertRaises(RuntimeError): candidate.check_identity(expected,record,changed,'today',True)
        with self.assertRaises(RuntimeError): candidate.check_identity(expected,record,owner,'yesterday',True)
        with self.assertRaises(RuntimeError): candidate.check_identity(expected,record,owner,'today',False)
        with self.assertRaises(RuntimeError): candidate.check_identity(expected,dict(record,data_root='/B'),owner,'today',True)

if __name__ == '__main__': unittest.main()
