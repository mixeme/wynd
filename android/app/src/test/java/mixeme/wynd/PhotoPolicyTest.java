package mixeme.wynd;

import org.junit.Test;
import static org.junit.Assert.*;

public final class PhotoPolicyTest {
    @Test public void nonceIsSingleUseExpiringAndClearedByNavigation() {
        PhotoRequestNonce request = new PhotoRequestNonce();
        String token = request.issue(100);
        assertTrue(request.consume(token, 101)); assertFalse(request.consume(token, 102));
        token = request.issue(100); assertFalse(request.consume(token, 10101));
        token = request.issue(100); request.clear(); assertFalse(request.consume(token, 101));
        token = request.issue(100); assertFalse(request.consume(token, 99));
        token = request.issue(100); assertFalse(request.consume("not-a-token", 101)); assertFalse(request.consume(token, 102));
    }
    @Test public void tokensAndFiltersAreNotArbitraryFiles() {
        for (String value : new String[] {null, "", "../private", "#fff", "image/jpeg\n"}) assertFalse(PhotoPolicy.token(value));
        assertTrue(PhotoPolicy.mediaMime("image/*")); assertTrue(PhotoPolicy.mediaMime("video/mp4"));
        assertFalse(PhotoPolicy.mediaMime("application/pdf")); assertFalse(PhotoPolicy.mediaMime("image/"));
        assertTrue(PhotoPolicy.localMedia("com.android.externalstorage.documents"));
        assertFalse(PhotoPolicy.localMedia("other.provider"));
    }
    @Test public void registryIsOpaqueScopedAndDoesNotExposeArbitraryUris() {
        SelectedMediaRegistry registry = new SelectedMediaRegistry();
        String a = registry.register(new SelectedMediaRegistry.Entry("page-a", "content://media/1", "a.jpg", "image/jpeg", 1));
        String b = registry.register(new SelectedMediaRegistry.Entry("page-b", "content://media/2", "b.jpg", "image/jpeg", 2));
        assertNotEquals(a, b); assertNull(registry.get("content://private/secret")); assertNull(registry.get(a + "/../secret"));
        registry.clear("page-a"); assertNull(registry.get(a)); assertEquals("b.jpg", registry.get(b).name);
    }
}
