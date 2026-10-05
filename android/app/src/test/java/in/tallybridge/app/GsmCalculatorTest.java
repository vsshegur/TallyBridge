package in.tallybridge.app;

import org.junit.Test;
import static org.junit.Assert.*;

public class GsmCalculatorTest {
 @Test public void calculatesInches(){assertEquals(969L,GsmCalculator.rounded(10,16,100,"in"));}
 @Test public void calculatesCentimetres(){assertEquals(510L,GsmCalculator.rounded(70,140,500,"cm"));}
 @Test(expected=IllegalArgumentException.class) public void rejectsZero(){GsmCalculator.calculate(0,16,100,"in");}
}
