import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Disabled;
import static org.junit.jupiter.api.Assertions.assertEquals;

class ReportTest {
    @Test void pass() {
        System.out.println("synthetic stdout");
        System.err.println("synthetic stderr");
    }
    @Test void fail() { assertEquals(1, 2, "synthetic failure"); }
    @Test void error() { throw new IllegalStateException("synthetic runtime error"); }
    @Test @Disabled("synthetic skip") void skip() {}
}
