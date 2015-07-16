package com.brotherlogic.recordcollection;
import com.brotherlogic.discogs.backend.UrlRetriever;

import java.io.IOException;

import org.apache.log4j.Level;
import org.apache.log4j.Logger;

import org.scribe.oauth.OAuthService;
import org.scribe.model.OAuthRequest;
import org.scribe.model.Response;
import org.scribe.model.Token;
import org.scribe.model.Verb;

public class ScribeRetriever implements UrlRetriever {
  
  Token authToken;
  OAuthService authService;
  RequestBuilder builder;
  
  Logger logger = Logger.getLogger(getClass());
  
  public ScribeRetriever(Token token, OAuthService service, RequestBuilder build) {
    authToken = token;
    authService = service;
    builder = build;
  }
  
  @Override
  public String get(String relativePath) throws IOException {
    logger.log(Level.INFO,"Building " + relativePath + " with " + builder);
    OAuthRequest request = builder.buildRequest(Verb.GET, "https://api.discogs.com/" + relativePath);
    request.addHeader("User-Agent", DiscogsApi.USER_AGENT);
    authService.signRequest(authToken, request);
    Response response = request.send();
    return response.getBody();
  }
}
