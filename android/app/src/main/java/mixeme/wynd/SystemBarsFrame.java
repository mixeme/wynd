package mixeme.wynd;

import android.annotation.SuppressLint;
import android.app.Activity;
import android.content.SharedPreferences;
import android.graphics.Canvas;
import android.graphics.Paint;
import android.os.Build;
import android.view.View;
import android.view.Window;
import android.view.WindowInsets;
import android.view.WindowInsetsController;
import android.webkit.WebView;
import android.widget.FrameLayout;
import org.json.JSONException;
import org.json.JSONObject;
import java.util.Locale;

/** На Android 15+ фон полос рисуется под прозрачными insets; веб-экран остаётся внутри них. */
@SuppressLint("ViewConstructor") // Только программное создание с готовым WebView; из XML не используется.
final class SystemBarsFrame extends FrameLayout {
    private final Activity activity;
    private final WebView web;
    private final SharedPreferences preferences;
    private final Paint statusPaint = new Paint();
    private final Paint navigationPaint = new Paint();
    private String status;
    private String navigation;
    private String requestedStatus;
    private String requestedNavigation;
    private int statusHeight;
    private int navigationHeight;

    SystemBarsFrame(Activity activity, WebView web) {
        super(activity);
        this.activity = activity;
        this.web = web;
        preferences = activity.getSharedPreferences("appearance", Activity.MODE_PRIVATE);
        String saved = preferences.getString("navigation", BarColors.PAPER);
        if (!BarColors.valid(saved)) saved = BarColors.PAPER;
        addView(web, new LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT));
        setWillNotDraw(false);
        if (Build.VERSION.SDK_INT >= 30) {
            activity.getWindow().setDecorFitsSystemWindows(false);
            setOnApplyWindowInsetsListener((view, insets) -> {
                android.graphics.Insets bars = insets.getInsets(WindowInsets.Type.systemBars() | WindowInsets.Type.displayCutout());
                android.graphics.Insets ime = insets.getInsets(WindowInsets.Type.ime());
                statusHeight = bars.top;
                navigationHeight = bars.bottom;
                LayoutParams params = (LayoutParams) web.getLayoutParams();
                params.setMargins(bars.left, bars.top, bars.right, Math.max(bars.bottom, ime.bottom));
                web.setLayoutParams(params);
                invalidate();
                return WindowInsets.CONSUMED;
            });
        }
        colors(saved, saved);
    }

    @Override protected void onAttachedToWindow() {
        super.onAttachedToWindow();
        colors(requestedStatus, requestedNavigation);
        requestApplyInsets();
    }

    @SuppressWarnings("deprecation")
    void colors(String requestedStatus, String requestedNavigation) {
        if (!BarColors.valid(requestedStatus) || !BarColors.valid(requestedNavigation)) return;
        this.requestedStatus = requestedStatus;
        this.requestedNavigation = requestedNavigation;
        status = BarColors.compatible(requestedStatus, Build.VERSION.SDK_INT, true);
        navigation = BarColors.compatible(requestedNavigation, Build.VERSION.SDK_INT, false);
        int top = BarColors.argb(status), bottom = BarColors.argb(navigation);
        statusPaint.setColor(top);
        navigationPaint.setColor(bottom);
        setBackgroundColor(bottom);
        web.setBackgroundColor(bottom);
        Window window = activity.getWindow();
        if (Build.VERSION.SDK_INT < 35) {
            window.setStatusBarColor(top);
            window.setNavigationBarColor(bottom);
        }
        if (Build.VERSION.SDK_INT >= 29) {
            window.setStatusBarContrastEnforced(false);
            window.setNavigationBarContrastEnforced(false);
        }
        boolean lightStatus = BarColors.darkIcons(status), lightNavigation = BarColors.darkIcons(navigation);
        if (Build.VERSION.SDK_INT >= 30) {
            WindowInsetsController controller = window.getInsetsController();
            if (controller != null) controller.setSystemBarsAppearance(
                    (lightStatus ? WindowInsetsController.APPEARANCE_LIGHT_STATUS_BARS : 0)
                            | (lightNavigation ? WindowInsetsController.APPEARANCE_LIGHT_NAVIGATION_BARS : 0),
                    WindowInsetsController.APPEARANCE_LIGHT_STATUS_BARS | WindowInsetsController.APPEARANCE_LIGHT_NAVIGATION_BARS);
        } else {
            View decor = window.getDecorView();
            int flags = decor.getSystemUiVisibility();
            if (Build.VERSION.SDK_INT >= 23) flags = lightStatus ? flags | View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR : flags & ~View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR;
            if (Build.VERSION.SDK_INT >= 26) flags = lightNavigation ? flags | View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR : flags & ~View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR;
            decor.setSystemUiVisibility(flags);
        }
        if (!requestedNavigation.equals(preferences.getString("navigation", ""))) {
            preferences.edit().putString("navigation", requestedNavigation).apply();
        }
        invalidate();
    }

    @Override protected void onDraw(Canvas canvas) {
        super.onDraw(canvas);
        canvas.drawRect(0, 0, getWidth(), statusHeight, statusPaint);
        canvas.drawRect(0, getHeight() - navigationHeight, getWidth(), getHeight(), navigationPaint);
    }

    @SuppressWarnings("deprecation")
    JSONObject state() throws JSONException {
        Window window = activity.getWindow();
        int top = Build.VERSION.SDK_INT < 35 ? window.getStatusBarColor() : statusPaint.getColor();
        int bottom = Build.VERSION.SDK_INT < 35 ? window.getNavigationBarColor() : navigationPaint.getColor();
        return new JSONObject().put("status", String.format(Locale.ROOT, "#%06X", top & 0xffffff))
                .put("navigation", String.format(Locale.ROOT, "#%06X", bottom & 0xffffff))
                .put("api", Build.VERSION.SDK_INT).put("paintedInsets", Build.VERSION.SDK_INT >= 35);
    }
}
