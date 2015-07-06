package com.brotherlogic.recordcollection;

import org.junit.Assert;
import org.junit.Test;

import org.mockito.Mockito;

import org.scribe.model.OAuthRequest;
import org.scribe.model.Verb;

public class DiscogsTokenTest {

    @Test
    public void testBuildUserBackend() {
        DiscogsService serv = Mockito.mock(DiscogsService.class);
        DiscogsToken token = new DiscogsToken("TestKey", "TestSecret", serv);
        Assert.assertNotNull(token.getUserBackend(new RequestBuilder() {
                public OAuthRequest buildRequest(Verb v, String url) {
                    return null;
                }
            }));
    }
    
}
