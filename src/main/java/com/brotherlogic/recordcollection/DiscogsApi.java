package com.brotherlogic.recordcollection;

import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.net.URL;
import java.net.HttpURLConnection;

import org.apache.log4j.Level;
import org.apache.log4j.Logger;

import org.scribe.builder.api.DefaultApi10a;
import org.scribe.extractors.HeaderExtractor;
import org.scribe.extractors.HeaderExtractorImpl;
import org.scribe.model.OAuthConfig;
import org.scribe.model.OAuthRequest;
import org.scribe.model.Request;
import org.scribe.model.RequestTuner;
import org.scribe.model.Token;
import org.scribe.oauth.OAuthService;
import org.scribe.oauth.OAuth10aServiceImpl;

public class DiscogsApi extends DefaultApi10a {

    private Logger logger = Logger.getLogger(getClass());

    public static final String USER_AGENT = "DiscogsSimonRecordCollection/0.1 +http://github.com/BrotherLogic/recordcollection";

    @Override
    public OAuthService createService(OAuthConfig config) {
        return new DiscogsService(this, config);
    }
    @Override
    public String getAccessTokenEndpoint() {
	return "https://api.discogs.com/oauth/access_token";
    }
    
    @Override
    public String getAuthorizationUrl(final Token arg0) {
	return String.format(
			     "https://www.discogs.com/oauth/authorize?oauth_token=%s",
			     arg0.getToken());
    }
    
    @Override
    public String getRequestTokenEndpoint() {
	return "https://api.discogs.com/oauth/request_token";
    }
}
