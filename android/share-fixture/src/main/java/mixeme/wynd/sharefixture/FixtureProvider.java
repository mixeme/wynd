package mixeme.wynd.sharefixture;

import android.content.ContentProvider;
import android.content.ContentValues;
import android.database.Cursor;
import android.database.MatrixCursor;
import android.net.Uri;
import android.os.ParcelFileDescriptor;
import android.provider.OpenableColumns;
import java.io.*;

public final class FixtureProvider extends ContentProvider {
    private String filename(Uri uri) {
        if (!"mixeme.wynd.sharefixture.media".equals(uri.getAuthority()) || uri.getQuery() != null) return null;
        if ("/photo".equals(uri.getPath())) return "wynd-share-fixture.jpg";
        if ("/video".equals(uri.getPath())) return "wynd-share-fixture.webm";
        return null;
    }
    @Override public boolean onCreate() { return true; }
    @Override public String getType(Uri uri) { return "/photo".equals(uri.getPath()) ? "image/jpeg" : "video/webm"; }
    @Override public Cursor query(Uri uri, String[] projection, String selection, String[] args, String sort) {
        String name = filename(uri); if (name == null) return null;
        MatrixCursor cursor = new MatrixCursor(new String[]{OpenableColumns.DISPLAY_NAME}); cursor.addRow(new Object[]{name}); return cursor;
    }
    @Override public ParcelFileDescriptor openFile(Uri uri, String mode) throws FileNotFoundException {
        String name = filename(uri); if (name == null || !"r".equals(mode) || getContext() == null) throw new FileNotFoundException();
        File file = new File(getContext().getCacheDir(), name);
        try {
            try (InputStream input = getContext().getAssets().open(name); OutputStream output = new FileOutputStream(file)) {
                byte[] buffer = new byte[4096]; int n; while ((n = input.read(buffer)) != -1) output.write(buffer, 0, n);
            }
            return ParcelFileDescriptor.open(file, ParcelFileDescriptor.MODE_READ_ONLY);
        } catch (IOException e) { throw new FileNotFoundException(); }
    }
    @Override public Uri insert(Uri u, ContentValues v) { throw new UnsupportedOperationException(); }
    @Override public int update(Uri u, ContentValues v, String s, String[] a) { throw new UnsupportedOperationException(); }
    @Override public int delete(Uri u, String s, String[] a) { throw new UnsupportedOperationException(); }
}
