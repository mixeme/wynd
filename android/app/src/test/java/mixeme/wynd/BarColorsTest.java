package mixeme.wynd;

import org.junit.Test;
import static org.junit.Assert.*;

public final class BarColorsTest {
    @Test public void acceptsOnlyOpaqueRgb() {
        assertEquals(0xfff4f0e9, BarColors.argb(BarColors.PAPER));
        for (String value : new String[] {null, "", "red", "transparent", "#fff", "#F4F0E900", "#FFFFFF\n", "javascript:alert(1)"}) {
            assertFalse(String.valueOf(value), BarColors.valid(value));
        }
    }

    @Test public void usesContrastingIconsForBothWyndThemes() {
        assertTrue(BarColors.darkIcons(BarColors.PAPER));
        assertFalse(BarColors.darkIcons(BarColors.DARK));
        assertTrue(BarColors.darkIcons("#FFFFFF"));
        assertFalse(BarColors.darkIcons("#000000"));
    }

    @Test public void oldAndroidRetainsReadableBarsWithoutDarkIconSupport() {
        assertEquals(BarColors.DARK, BarColors.compatible(BarColors.PAPER, 21, true));
        assertEquals(BarColors.PAPER, BarColors.compatible(BarColors.PAPER, 23, true));
        assertEquals(BarColors.DARK, BarColors.compatible(BarColors.PAPER, 25, false));
        assertEquals(BarColors.PAPER, BarColors.compatible(BarColors.PAPER, 26, false));
        assertEquals(BarColors.DARK, BarColors.compatible(BarColors.DARK, 21, false));
    }
}
