package com.brotherlogic.recordcollection;

import java.util.Map;
import java.util.TreeMap;

import org.apache.log4j.Level;
import org.apache.log4j.Logger;

import org.junit.Test;
import org.junit.Assert;

import org.mockito.Mockito;

import org.scribe.builder.ServiceBuilder;
import org.scribe.extractors.HeaderExtractor;
import org.scribe.model.OAuthConfig;
import org.scribe.model.OAuthRequest;
import org.scribe.model.Token;
import org.scribe.model.Verb;
import org.scribe.oauth.OAuthService;

public class DiscogsApiTest extends BaseTest {

    Logger logger = Logger.getLogger(getClass());

    @Test
    public void testHeaderAdded() {
        OAuthService service = new ServiceBuilder()
            .provider(DiscogsApi.class)
            .apiKey("test-key")
            .apiSecret("test-secret")
            .build();

        OAuthRequest request = new OAuthRequest(Verb.GET,"http://test");
        Token token = new Token("madeup1","madeup2");
        
        service.signRequest(token,request);
    }
    
    @Test
    public void testAuthorizationURL(){
        Token t = Mockito.mock(Token.class);
        Mockito.when(t.getToken()).thenReturn("test");

        DiscogsApi api = new DiscogsApi();
        String authURL = api.getAuthorizationUrl(t);
        Assert.assertEquals("https://www.discogs.com/oauth/authorize?oauth_token=test",authURL);
    }

    @Test
    public void testTokenEndPoint() {
        DiscogsApi api = new DiscogsApi();
        Assert.assertEquals("https://api.discogs.com/oauth/request_token",api.getRequestTokenEndpoint());
    }

    @Test
    public void testGetAccessTokenEndpoint() {
        DiscogsApi api = new DiscogsApi();
        Assert.assertEquals("https://api.discogs.com/oauth/access_token",api.getAccessTokenEndpoint());
    }
}
