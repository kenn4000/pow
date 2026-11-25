import { mkdirSync, copyFileSync, writeFileSync } from 'node:fs';
const root = 'android/app/src/main';
mkdirSync(root + '/res/drawable-nodpi', { recursive: true });
copyFileSync('assets/studio-splash.png', root + '/res/drawable-nodpi/studio_splash.png');
copyFileSync('assets/game-loading.png', root + '/res/drawable-nodpi/game_loading.png');
const dir = root + '/java/com/markdownthing/prisonersofwar';
mkdirSync(dir, { recursive: true });
writeFileSync(dir + '/MainActivity.java', `package com.markdownthing.prisonersofwar;

import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.os.SystemClock;
import android.graphics.Color;
import android.view.ViewGroup;
import android.widget.FrameLayout;
import android.widget.ImageView;
import com.getcapacitor.BridgeActivity;

public class MainActivity extends BridgeActivity {
    private final Handler handler = new Handler(Looper.getMainLooper());
    private FrameLayout overlay;
    private long start;
    @Override public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        if (savedInstanceState != null) return;
        start = SystemClock.elapsedRealtime();
        overlay = new FrameLayout(this);
        overlay.setBackgroundColor(Color.BLACK);
        overlay.setClickable(true);
        ImageView art = new ImageView(this);
        art.setScaleType(ImageView.ScaleType.FIT_CENTER);
        art.setImageResource(R.drawable.studio_splash);
        overlay.addView(art, new FrameLayout.LayoutParams(-1, -1));
        addContentView(overlay, new ViewGroup.LayoutParams(-1, -1));
        handler.postDelayed(() -> {
            if (isFinishing() || isDestroyed()) return;
            art.setImageResource(R.drawable.game_loading);
            handler.postDelayed(this::checkReady, 1000);
        }, 1500);
    }
    private void checkReady() {
        if (isFinishing() || isDestroyed() || overlay == null) return;
        boolean loaded = getBridge() != null && getBridge().getWebView() != null
            && getBridge().getWebView().getProgress() >= 100;
        if (loaded || SystemClock.elapsedRealtime() - start >= 20000) {
            if (overlay.getParent() instanceof ViewGroup) {
                ((ViewGroup) overlay.getParent()).removeView(overlay);
            }
            overlay = null;
        } else handler.postDelayed(this::checkReady, 250);
    }
    @Override public void onDestroy() {
        handler.removeCallbacksAndMessages(null);
        super.onDestroy();
    }
}
`);
