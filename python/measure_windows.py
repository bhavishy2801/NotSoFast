"""Five repeated small benchmark matrices with Windows job CPU accounting.

Job CPU includes all descendant processes, including exited Git processes.
RSS is sampled, not an exact peak; committed memory is a separate job metric.
Test-only dependencies: pywin32, psutil. Build .tools/bench.exe from ./core first.
"""
import json
import os
from pathlib import Path
import statistics
import subprocess
import time

import psutil
import win32api
import win32event
import win32job
import win32process

ROOT=Path(__file__).resolve().parents[1]


def trial(index):
    output=ROOT/'docs'/f'repeated-run-{index}.json'
    env={**os.environ,'NSF_BENCH':'1','NSF_BENCH_SMALL':'1','NSF_BENCH_OUT':str(output)}
    job=win32job.CreateJobObject(None, "")
    limits=win32job.QueryInformationJobObject(job,win32job.JobObjectExtendedLimitInformation)
    limits['BasicLimitInformation']['LimitFlags']=win32job.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
    win32job.SetInformationJobObject(job,win32job.JobObjectExtendedLimitInformation,limits)
    process=thread=None
    start=time.perf_counter();peak_rss=0;samples=0
    try:
        process,thread,pid,_=win32process.CreateProcess(None,subprocess.list2cmdline([str(ROOT/'.tools/bench.exe'),'-test.run=^TestEarlyBenchmark$','-test.timeout=10m']),
                           None,None,False,win32process.CREATE_SUSPENDED|win32process.CREATE_NO_WINDOW,env,str(ROOT),win32process.STARTUPINFO())
        win32job.AssignProcessToJobObject(job,process)
        win32process.ResumeThread(thread)
        parent=psutil.Process(pid)
        while win32event.WaitForSingleObject(process,10)==win32event.WAIT_TIMEOUT:
            rss=0
            try:
                for child in [parent]+parent.children(recursive=True):
                    try:rss+=child.memory_info().rss
                    except (psutil.NoSuchProcess,psutil.AccessDenied):pass
            except (psutil.NoSuchProcess,psutil.AccessDenied):pass
            peak_rss=max(peak_rss,rss);samples+=1
        code=win32process.GetExitCodeProcess(process)
        if code:raise RuntimeError(f'benchmark exited {code}')
        accounting=win32job.QueryInformationJobObject(job,win32job.JobObjectBasicAccountingInformation)
        memory=win32job.QueryInformationJobObject(job,win32job.JobObjectExtendedLimitInformation)
        row={'run':index,'wall_seconds':time.perf_counter()-start,'job_cpu_seconds':(accounting['TotalUserTime']+accounting['TotalKernelTime'])/10_000_000,
             'total_processes':accounting['TotalProcesses'],'job_peak_committed_bytes':memory['PeakJobMemoryUsed'],
             'sampled_peak_aggregate_rss_bytes':peak_rss,'rss_samples':samples,'sample_wait_ms':10,'rows_file':output.name}
        print(json.dumps(row),flush=True);return row
    finally:
        if thread:win32api.CloseHandle(thread)
        if process:win32api.CloseHandle(process)
        win32api.CloseHandle(job)


if __name__=='__main__':
    resources=[trial(i) for i in range(1,6)]
    groups={}
    for i in range(1,6):
        for row in json.loads((ROOT/'docs'/f'repeated-run-{i}.json').read_text()):
            key=(row['predicate'],row['change'],row['method'])
            groups.setdefault(key,[]).append(row['total_ns']/1e6)
    summary=[dict(predicate=k[0],change=k[1],method=k[2],n=len(v),median_ms=statistics.median(v),min_ms=min(v),max_ms=max(v)) for k,v in groups.items()]
    result={'method':'Five independent 32-file matrices, fixed method order, warm OS caches, app-cache state reset identically per baseline. Resource totals include fixture setup and all benchmark cases; not per-action cost. RSS sampling misses short-lived peaks. Job committed memory is not RSS.',
            'resources':resources,'latency_summary':summary}
    (ROOT/'docs/repeated-results.json').write_text(json.dumps(result,indent=2))

