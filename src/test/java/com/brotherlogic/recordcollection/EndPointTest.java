package com.brotherlogic.recordcollection;

import java.util.Map;
import java.util.TreeMap;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.StringReader;

import com.brotherlogic.discogs.User;
import com.brotherlogic.discogs.backend.UserBackend;


import com.google.gson.JsonParser;
import com.google.gson.JsonElement;

import org.apache.log4j.Level;
import org.apache.log4j.Logger;

import org.junit.Assert;
import org.junit.Test;

import org.mockito.Mockito;

import org.scribe.oauth.OAuthService;
import org.scribe.model.Token;
import org.scribe.model.Verifier;

import javax.servlet.ServletContext;
import javax.servlet.ServletOutputStream;
import javax.servlet.WriteListener;
import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpServletResponse;

public class EndPointTest extends BaseTest {

    private Logger logger = Logger.getLogger(getClass());

    private Map<String,DiscogsToken> authTokens = new TreeMap<String,DiscogsToken>();

    private ServletContext mContext;
    
    private HttpServletResponse testRequest(String requestURL) throws Exception {
        return testRequest(requestURL, null);
    }

    private HttpServletResponse testRequest(String requestURL, Token tokenInMap) throws Exception {
        return testRequest(requestURL, tokenInMap, null);
    }
    
    private HttpServletResponse testRequest(String requestURL, Token tokenInMap, Token authToken) throws Exception{
        HttpServletResponse response = Mockito.mock(HttpServletResponse.class);
        HttpServletRequest request = Mockito.mock(HttpServletRequest.class);
        
        //Setup the request to be a login
        Mockito.when(request.getRequestURI()).thenReturn("/endpoint" + requestURL);
        Mockito.when(request.getRequestURL()).thenReturn(new StringBuffer("http://localhost:8080/endpoint" + requestURL));
        
        //Setup so we can read out the outputstream
        ByteArrayMockOutputStream bamos = new ByteArrayMockOutputStream();
        Mockito.when(response.getOutputStream()).thenReturn(bamos);

        EndPoint myEndPoint = new EndPoint();
        DiscogsService mService = Mockito.mock(DiscogsService.class);
        Token mToken = new Token("TestToken","TestSecret");
        Token mAuthToken = new Token("TestToken","SecretAuth");
        logger.log(Level.INFO,"Should get " + mAuthToken + " from " + mToken);
        Mockito.when(mService.getAccessToken(Mockito.eq(mToken), Mockito.any(Verifier.class))).thenReturn(mAuthToken);
        mContext = Mockito.mock(ServletContext.class);
        Mockito.when(mContext.getAttribute("auth_tokens")).thenReturn(authTokens);
        
        Map<String,Token> initTokenMap = new TreeMap<String,Token>();
        if (tokenInMap != null)
            initTokenMap.put("TestToken", tokenInMap);
        
        Mockito.when(mService.getRequestToken()).thenReturn(mToken);
        Mockito.when(mService.getAuthorizationUrl(mToken)).thenReturn("http://initialauth");
        Mockito.when(request.getServletContext()).thenReturn(mContext);
        Mockito.when(mContext.getAttribute("config")).thenReturn(new Config("testkey","testsecret",mService));
        Mockito.when(mContext.getAttribute("token_map")).thenReturn(initTokenMap);

        if (requestURL.contains("?"))
            for(String mapper : requestURL.substring(requestURL.indexOf("?")+1).split("&")) {
                String[] bits = mapper.split("=");
                logger.log(Level.DEBUG,"Putting " + bits[0] + " -> " + bits[1] + " into parameter map");
                Mockito.when(request.getParameter(bits[0])).thenReturn(bits[1]);
            }
        
        myEndPoint.service(request,response);

        //Check that the response is as expected
        return response;
    }

    @Test
    public void testEmptyRequest() throws Exception {
        String response = ((ByteArrayMockOutputStream) testRequest("/").getOutputStream()).getString();
        JsonElement elem = new JsonParser().parse(response);
        Assert.assertTrue(elem.isJsonNull());
    }

    @Test
    public void testUnmatchedRequest() throws Exception {
        String response = ((ByteArrayMockOutputStream) testRequest("/thisisagibberishrequest/").getOutputStream()).getString();
        JsonElement elem = new JsonParser().parse(response);
        Assert.assertTrue(elem.isJsonNull());
    }
    
    @Test
    public void testLoginRequest() throws Exception {
        String responseString = ((ByteArrayMockOutputStream) testRequest("/login/").getOutputStream()).getString();
        logger.log(Level.DEBUG,"Login response = " + responseString);
        Assert.assertTrue(responseString.contains("redirect"));
    }

    @Test
    public void testCallbackRequest() throws Exception {
        HttpServletResponse response = testRequest("/callback?oauth_token=TestToken&oauth_verifier=TestVerifier", new Token("TestToken","TestSecret"));
        Mockito.verify(response).sendRedirect("/index.html?token=TestToken");
    }

    @Test
    public void testRetrieveMeWithoutTokenCausesRedirectToLoginWithRemoveCookie() throws Exception {
        HttpServletResponse response = testRequest("/me?token=TestAuth",null,null);
        String responseStr = ((ByteArrayMockOutputStream) response.getOutputStream()).getString();
        Assert.assertTrue(responseStr.contains("redirect"));
    }

    @Test
    public void testRetrieveMe() throws Exception {
        DiscogsToken authToken = Mockito.mock(DiscogsToken.class);
        User u = new User("brotherlogic");
        UserBackend backend = Mockito.mock(UserBackend.class);
        Mockito.when(backend.getMe()).thenReturn(u);
        Mockito.when(authToken.getUserBackend(Mockito.any(RequestBuilder.class))).thenReturn(backend);
        authTokens.put("TestAuth",authToken);
        
        String response = ((ByteArrayMockOutputStream) testRequest("/me?token=TestAuth",null,authToken).getOutputStream()).getString();
        logger.log(Level.DEBUG,"Retrieve Me response = " + response);
        Assert.assertTrue(response.contains("brotherlogic"));
    }
}

class ByteArrayMockOutputStream extends ServletOutputStream {
    ByteArrayOutputStream baos = new ByteArrayOutputStream();

    public boolean isReady() {
        return true;
    }
    
    public String getString() {
        try{
            return baos.toString("UTF-8");
        } catch (IOException e) {
            e.printStackTrace();
            return "";
        }
    }
    
    public void close() throws IOException {
        baos.close();
    }

    public void flush() throws IOException {
        baos.flush();
    }

    public void write(byte[] b) throws IOException {
        baos.write(b);
    }

    public void write(byte[] b, int off, int len) throws IOException {
        baos.write(b,off,len);
    }

    public void write(int b) throws IOException {
        baos.write(b);
    }

    public void setWriteListener(WriteListener listener) {
        //Ignore this
    }
}
