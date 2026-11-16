package mixeme.wynd;

/** Непрозрачные RGB из Wynd; светлая полоса требует поддерживаемых тёмных значков. */
final class BarColors {
    static final String PAPER = "#F4F0E9";
    static final String DARK = "#211E1C";
    static boolean valid(String color) { return color != null && color.matches("#[0-9a-fA-F]{6}"); }

    static int argb(String color) {
        if (!valid(color)) throw new IllegalArgumentException("Expected opaque RGB");
        return 0xff000000 | Integer.parseInt(color.substring(1), 16);
    }

    static boolean darkIcons(String color) {
        int rgb = argb(color);
        double luminance = 0.2126 * linear((rgb >> 16) & 255)
                + 0.7152 * linear((rgb >> 8) & 255) + 0.0722 * linear(rgb & 255);
        return luminance > 0.179;
    }

    static String compatible(String color, int api, boolean statusBar) {
        return api < (statusBar ? 23 : 26) && darkIcons(color) ? DARK : color;
    }

    private static double linear(int value) {
        double channel = value / 255.0;
        return channel <= 0.04045 ? channel / 12.92 : Math.pow((channel + 0.055) / 1.055, 2.4);
    }
}
