"""Explicit developer-only capture of synthetic producer output, never used by import/tests."""
import pathlib
import re
import shutil
import socket
import subprocess
import tempfile

ROOT = pathlib.Path(__file__).resolve().parents[1]
DEST = ROOT / "internal/gotestreport/testdata/junit"
SOURCE = DEST / "source"


def run(argv, cwd, expected=0):
    result = subprocess.run(argv, cwd=cwd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    if result.returncode != expected:
        raise RuntimeError(f"{argv}: expected {expected}, got {result.returncode}\\n{result.stdout.decode()}")


def capture(source, name, work):
    # Only machine-specific paths/hostname are normalized. Do not reconstruct XML.
    text = source.read_text().replace(str(work.resolve()), "/synthetic/junit").replace(str(work), "/synthetic/junit")
    text = text.replace(socket.gethostname(), "synthetic-host")
    # Surefire emits the complete JVM/system property inventory. Strip that
    # metadata block rather than committing local paths or ambient settings.
    text = re.sub(r"  <properties>.*?</properties>\n", "", text, flags=re.S)
    (DEST / name).write_text(text)


with tempfile.TemporaryDirectory(prefix="after-junit-capture-") as directory:
    work = pathlib.Path(directory)
    shutil.copytree(SOURCE, work, dirs_exist_ok=True)
    pytest = ["mise", "exec", "uv", "--", "uv", "run", "--no-project", "--with", "pytest==8.4.2", "pytest"]
    run(pytest + ["test_report.py", "--junitxml=pytest.xml", "-o", "junit_logging=all", "-q"], work, 1)
    capture(work / "pytest.xml", "pytest.xml", work)
    run(pytest + ["test_report.py", "-k", "no_such_test", "--junitxml=pytest-empty.xml", "-q"], work, 5)
    capture(work / "pytest-empty.xml", "pytest-empty.xml", work)
    run(["bun", "install"], work)
    run(["bunx", "--no-install", "vitest", "run", "--reporter=junit", "--outputFile=vitest.xml"], work, 1)
    capture(work / "vitest.xml", "vitest.xml", work)
    run(["bun", "test", "./bun_report_test.js", "--reporter=junit", "--reporter-outfile=bun.xml"], work, 1)
    capture(work / "bun.xml", "bun.xml", work)
    tests = work / "src/test/java"
    tests.mkdir(parents=True)
    shutil.copy(work / "ReportTest.java", tests)
    run(["mise", "exec", "java@temurin-21.0.8+9", "maven@3.9.11", "--", "mvn", "-B", "test"], work, 1)
    capture(work / "target/surefire-reports/TEST-ReportTest.xml", "surefire.xml", work)
print("Captured pytest 8.4.2, Vitest 3.2.4, Bun 1.4.2, Maven Surefire 3.5.4 / JUnit Jupiter 5.13.4 (Maven 3.9.11, Temurin 21.0.8+9).")
