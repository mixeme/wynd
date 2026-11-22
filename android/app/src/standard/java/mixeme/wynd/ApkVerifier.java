package mixeme.wynd;

import android.content.Context;
import android.content.pm.PackageInfo;
import android.content.pm.PackageManager;
import android.content.pm.Signature;
import android.os.Build;

import java.io.File;
import java.io.FileInputStream;
import java.io.IOException;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.HashSet;
import java.util.Set;

/** Android проверяет подпись APK; SHA-256 выпуска сам по себе не удостоверяет автора. */
final class ApkVerifier {
    static final class Rejected extends IOException {
        final String code;
        Rejected(String code) { super(code); this.code = code; }
    }

    @SuppressWarnings("deprecation")
    static void verify(Context context, File file, ReleaseCatalog.Release release) throws Rejected {
        PackageManager manager = context.getPackageManager();
        // GET_SIGNATURES также заставляет старые PackageParser проверять содержимое,
        // а не только читать метаданные сертификата (AOSP Android 10, skipVerify=false).
        int flags = PackageManager.GET_SIGNATURES;
        if (Build.VERSION.SDK_INT >= 28) flags |= PackageManager.GET_SIGNING_CERTIFICATES;
        PackageInfo candidate;
        PackageInfo installed;
        try {
            candidate = manager.getPackageArchiveInfo(file.getAbsolutePath(), flags);
            installed = manager.getPackageInfo(context.getPackageName(), flags);
        } catch (PackageManager.NameNotFoundException | RuntimeException e) {
            throw new Rejected("signature");
        }
        if (candidate == null || signers(candidate).isEmpty()
                || !signers(candidate).equals(signers(installed))) throw new Rejected("signature");
        if (!context.getPackageName().equals(candidate.packageName)) throw new Rejected("package");
        long candidateCode = Build.VERSION.SDK_INT >= 28 ? candidate.getLongVersionCode() : candidate.versionCode;
        long installedCode = Build.VERSION.SDK_INT >= 28 ? installed.getLongVersionCode() : installed.versionCode;
        String expectedName = release.version + (BuildConfig.DEBUG ? "-debug" : "");
        if (candidateCode != release.code || candidateCode <= installedCode
                || !expectedName.equals(candidate.versionName)) throw new Rejected("version");
        if (Build.VERSION.SDK_INT >= 24 && candidate.applicationInfo != null
                && candidate.applicationInfo.minSdkVersion > Build.VERSION.SDK_INT) throw new Rejected("android");
    }

    @SuppressWarnings("deprecation")
    private static Set<String> signers(PackageInfo info) {
        Signature[] signatures = Build.VERSION.SDK_INT >= 28 && info.signingInfo != null
                ? info.signingInfo.getApkContentsSigners() : info.signatures;
        Set<String> result = new HashSet<>();
        if (signatures != null) for (Signature signature : signatures) result.add(hex(digest(signature.toByteArray())));
        return result;
    }

    static String sha256(File file) throws IOException {
        MessageDigest digest = sha256Digest();
        try (FileInputStream input = new FileInputStream(file)) {
            byte[] buffer = new byte[32768];
            int read;
            while ((read = input.read(buffer)) != -1) digest.update(buffer, 0, read);
        }
        return hex(digest.digest());
    }

    private static byte[] digest(byte[] value) { return sha256Digest().digest(value); }
    private static MessageDigest sha256Digest() {
        try { return MessageDigest.getInstance("SHA-256"); }
        catch (NoSuchAlgorithmException e) { throw new AssertionError(e); }
    }
    private static String hex(byte[] value) {
        StringBuilder out = new StringBuilder();
        for (byte b : value) {
            out.append(Character.forDigit((b >>> 4) & 15, 16));
            out.append(Character.forDigit(b & 15, 16));
        }
        return out.toString();
    }
}
