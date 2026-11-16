package mixeme.wynd;

import android.content.ContentProvider;
import android.content.ContentValues;
import android.database.Cursor;
import android.database.MatrixCursor;
import android.net.Uri;
import android.os.ParcelFileDescriptor;
import android.provider.OpenableColumns;
import java.io.FileNotFoundException;

/** Не экспортируется. WebView получает FD только оригинала, выбранного в этом окне. */
public final class SelectedMediaProvider extends ContentProvider {
    static final SelectedMediaRegistry REGISTRY = new SelectedMediaRegistry();
    @Override public boolean onCreate() { return true; }

    private SelectedMediaRegistry.Entry entry(Uri uri) {
        if (getContext() == null || !"content".equals(uri.getScheme())
                || !(getContext().getPackageName() + ".selected-media").equals(uri.getAuthority())
                || uri.getPathSegments().size() != 1 || uri.getQuery() != null || uri.getFragment() != null) return null;
        return REGISTRY.get(uri.getLastPathSegment());
    }
    @Override public String getType(Uri uri) {
        SelectedMediaRegistry.Entry item = entry(uri); return item == null ? null : item.mime;
    }
    @Override public Cursor query(Uri uri, String[] projection, String selection, String[] args, String order) {
        SelectedMediaRegistry.Entry item = entry(uri);
        if (item == null) return null;
        String[] columns = projection == null ? new String[] {OpenableColumns.DISPLAY_NAME, OpenableColumns.SIZE} : projection;
        MatrixCursor result = new MatrixCursor(columns, 1);
        Object[] row = new Object[columns.length];
        for (int i = 0; i < columns.length; i++) {
            if (OpenableColumns.DISPLAY_NAME.equals(columns[i])) row[i] = item.name;
            else if (OpenableColumns.SIZE.equals(columns[i]) && item.size >= 0) row[i] = item.size;
        }
        result.addRow(row); return result;
    }
    @Override public ParcelFileDescriptor openFile(Uri uri, String mode) throws FileNotFoundException {
        SelectedMediaRegistry.Entry item = entry(uri);
        if (item == null || !"r".equals(mode) || getContext() == null) throw new FileNotFoundException("Selected media unavailable");
        try {
            ParcelFileDescriptor descriptor = getContext().getContentResolver().openFileDescriptor(Uri.parse(item.source), "r");
            if (descriptor == null) throw new FileNotFoundException("Selected media unavailable");
            return descriptor;
        } catch (SecurityException | UnsupportedOperationException e) { throw new FileNotFoundException("Original media unavailable"); }
    }
    @Override public Uri insert(Uri uri, ContentValues values) { throw new UnsupportedOperationException("Read only"); }
    @Override public int update(Uri uri, ContentValues values, String selection, String[] args) { throw new UnsupportedOperationException("Read only"); }
    @Override public int delete(Uri uri, String selection, String[] args) { throw new UnsupportedOperationException("Read only"); }
}
