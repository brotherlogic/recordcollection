package com.brotherlogic.recordcollection;

import com.brotherlogic.recordcollection.storage.Storage;

import java.util.Arrays;
import java.util.Collection;
import java.util.LinkedList;
import java.util.List;
import java.util.Map;
import java.util.TreeMap;
import java.io.BufferedReader;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.Reader;
import java.io.StringReader;

import com.brotherlogic.discogs.Folder;
import com.brotherlogic.discogs.User;
import com.brotherlogic.discogs.backend.CollectionBackend;
import com.brotherlogic.discogs.backend.UserBackend;

import com.google.gson.Gson;
import com.google.gson.JsonArray;
import com.google.gson.JsonParser;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonPrimitive;

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
  private Map<String,Token> authTokens = new TreeMap<String,Token>();
  private ServletContext mContext;
  private RecordCollection testCollection = new RecordCollection(Arrays.asList(new Integer[] {12,23,24}), Arrays.asList(new Integer[] {23,24,25}),"Testing");
  private List<RecordCollection> testCollections = Arrays.asList(new RecordCollection[]{new RecordCollection(Arrays.asList(new Integer[] {12,23}), Arrays.asList(new Integer[] {23,24}), "Testing"), new RecordCollection(Arrays.asList(new Integer[] {24,25}), Arrays.asList(new Integer[] {25,26}), "Testing2")});

  
  private HttpServletResponse testRequest(String requestURL) throws Exception {
    return testRequest(requestURL, null);
  }
  
  private HttpServletResponse testRequest(String requestURL, Token tokenInMap) throws Exception {
    return testRequest(requestURL, tokenInMap, null,"GET", null);
  }
  
  private HttpServletResponse testRequest(String type, String URL, JsonElement data, Token tokenInMap) throws Exception {
    return testRequest(URL, tokenInMap, tokenInMap, type, data);
  }
    
  private HttpServletResponse testRequest(String requestURL, Token tokenInMap, Token authToken, String requestType, JsonElement requestData) throws Exception{
    HttpServletResponse response = Mockito.mock(HttpServletResponse.class);
    HttpServletRequest request = Mockito.mock(HttpServletRequest.class);
    
    //Setup the request to be a login
    Mockito.when(request.getRequestURI()).thenReturn("/endpoint" + requestURL);
    Mockito.when(request.getRequestURL()).thenReturn(new StringBuffer("http://localhost:8080/endpoint" + requestURL));
    
    //Setup so we can read out the outputstream
    ByteArrayMockOutputStream bamos = new ByteArrayMockOutputStream();
    Mockito.when(response.getOutputStream()).thenReturn(bamos);

    Mockito.when(request.getMethod()).thenReturn(requestType);
    if (requestData != null) {
      Mockito.when(request.getReader()).thenReturn(new BufferedReader(new StringReader(requestData + "")));
    }
    
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
    
    RcSystem sys = Mockito.mock(RcSystem.class);
    Mockito.when(sys.getConfig()).thenReturn(new Config("testkey","testsecret",mService));
    Storage mStorage = Mockito.mock(Storage.class);
    
    for(String str : authTokens.keySet()) {
      logger.log(Level.INFO,"Putting " + authTokens.get(str) + " for " + str);
      Mockito.when(mStorage.getToken(str)).thenReturn(authTokens.get(str));
    }
    Mockito.when(mStorage.getCollection(12345,"Testing")).thenReturn(testCollection);
    Mockito.when(sys.getStorage()).thenReturn(mStorage);

    Mockito.when(mStorage.getCollections(12345)).thenReturn(testCollections);
    
    Mockito.when(mContext.getAttribute("system")).thenReturn(sys);
    Mockito.when(mContext.getAttribute("token_map")).thenReturn(initTokenMap);
    
    if (requestURL.contains("?"))
      for(String mapper : requestURL.substring(requestURL.indexOf("?")+1).split("&")) {
        String[] bits = mapper.split("=");
        logger.log(Level.DEBUG,"Putting " + bits[0] + " -> " + bits[1] + " into parameter map");
        Mockito.when(request.getParameter(bits[0])).thenReturn(bits[1]);
      }
    if (tokenInMap != null)
      myEndPoint.addUserId(new DiscogsToken("testtoken","",null), 12345);
    myEndPoint.service(request,response);
    
    //Check that the response is as expected
    return response;
  }

    @Test
    public void testEmptyRequest() throws Exception {
      Token authToken = Mockito.mock(Token.class);
      Mockito.when(authToken.getToken()).thenReturn("testtoken");
      Mockito.when(authToken.getSecret()).thenReturn("testsecret");
      authTokens.put("TestAuth",authToken);
      
      String response = ((ByteArrayMockOutputStream) testRequest("/?token=TestAuth").getOutputStream()).getString();
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
      HttpServletResponse response = testRequest("/me?token=TestAuth22");
      String responseStr = ((ByteArrayMockOutputStream) response.getOutputStream()).getString();
      Assert.assertTrue(responseStr.contains("redirect"));
    }

    @Test
    public void testOverviewRequest() throws Exception {
      DiscogsToken authToken = Mockito.mock(DiscogsToken.class);
      Mockito.when(authToken.getToken()).thenReturn("testtoken");
      Mockito.when(authToken.getSecret()).thenReturn("testsecret");
      
      Collection<Folder> folders = new LinkedList<Folder>();
      Folder f1 = Mockito.mock(Folder.class);
      Mockito.when(f1.getCount()).thenReturn(10);
      Folder f2 = Mockito.mock(Folder.class);
      Mockito.when(f2.getCount()).thenReturn(20);
      folders.add(f1);
      folders.add(f2);
      
      CollectionBackend backend = Mockito.mock(CollectionBackend.class);
      Mockito.when(backend.getFolders("brotherlogic")).thenReturn(folders);
      Mockito.when(authToken.getCollectionBackend(Mockito.any(RequestBuilder.class))).thenReturn(backend);
      authTokens.put("TestAuth",authToken);
      
      String responseString = ((ByteArrayMockOutputStream) testRequest("/overview/brotherlogic?token=TestAuth").getOutputStream()).getString();
      logger.log(Level.DEBUG,"Response = " + responseString);
      JsonObject obj = new JsonParser().parse(responseString).getAsJsonObject();
      Assert.assertEquals(2,obj.get("number_of_folders").getAsInt());
      Assert.assertEquals(30,obj.get("collection_size").getAsInt());
    }
    
    @Test
    public void testRetrieveMe() throws Exception {
      DiscogsToken authToken = Mockito.mock(DiscogsToken.class);
      Mockito.when(authToken.getToken()).thenReturn("testtoken");
      Mockito.when(authToken.getSecret()).thenReturn("testsecret");
      
      User u = new User("brotherlogic", 1234);
      UserBackend backend = Mockito.mock(UserBackend.class);
      Mockito.when(backend.getMe()).thenReturn(u);
      Mockito.when(authToken.getUserBackend(Mockito.any(RequestBuilder.class))).thenReturn(backend);
      authTokens.put("TestAuth",authToken);
      
      String response = ((ByteArrayMockOutputStream) testRequest("/me?token=TestAuth",null,authToken,null,null).getOutputStream()).getString();
      logger.log(Level.DEBUG,"Retrieve Me response = " + response);
      Assert.assertTrue(response.contains("brotherlogic"));
    }

  @Test
  public void testStoreCollection() throws Exception {
    DiscogsToken authToken = Mockito.mock(DiscogsToken.class);
    Mockito.when(authToken.getToken()).thenReturn("testtoken");
    Mockito.when(authToken.getSecret()).thenReturn("testsecret");
    authTokens.put("TestAuth",authToken);
    RecordCollection collection = new RecordCollection(Arrays.asList(new Integer[] {12,23,24}), Arrays.asList(new Integer[] {23,24,25}),"Testing");
    
    String response = ((ByteArrayMockOutputStream) testRequest("PUT","/collection?token=TestAuth",new Gson().toJsonTree(collection), authToken).getOutputStream()).getString();
    JsonObject obj = new JsonParser().parse(response).getAsJsonObject();
    Assert.assertTrue(obj.get("response").getAsBoolean());
  }

  @Test
  public void testRetrieveCollection() throws Exception {
    DiscogsToken authToken = Mockito.mock(DiscogsToken.class);
    Mockito.when(authToken.getToken()).thenReturn("testtoken");
    Mockito.when(authToken.getSecret()).thenReturn("testsecret");
    authTokens.put("TestAuth",authToken);
    
    String response = ((ByteArrayMockOutputStream) testRequest("/collection?token=TestAuth&name=Testing",authToken,authToken,"GET",null).getOutputStream()).getString();
    logger.log(Level.INFO, "Parsing: " + response);
    JsonObject obj = new JsonParser().parse(response).getAsJsonObject();
    Assert.assertEquals(obj,new Gson().toJsonTree(testCollection));
  }

  @Test
  public void testRetrieveCollections() throws Exception {
    DiscogsToken authToken = Mockito.mock(DiscogsToken.class);
    Mockito.when(authToken.getToken()).thenReturn("testtoken");
    Mockito.when(authToken.getSecret()).thenReturn("testsecret");
    authTokens.put("TestAuth",authToken);
    
    String response = ((ByteArrayMockOutputStream) testRequest("/collections?token=TestAuth&name=Testing",authToken,authToken,"GET",null).getOutputStream()).getString();
    logger.log(Level.INFO, "Parsing: " + response);
    JsonArray obj = new JsonParser().parse(response).getAsJsonArray();
    Assert.assertEquals(obj,new Gson().toJsonTree(testCollections));
  }

  
  @Test
  public void testGetUserIdWithNoMap() throws Exception {
    EndPoint ep = new EndPoint();
    DiscogsToken mToken = Mockito.mock(DiscogsToken.class);
    Mockito.when(mToken.getToken()).thenReturn("TestToken");
    User u = new User("TestUser",1234);
    DiscogsService mService = Mockito.mock(DiscogsService.class);
    UserBackend mBackend = Mockito.mock(UserBackend.class);
    Mockito.when(mBackend.getMe()).thenReturn(u);
    Mockito.when(mToken.getUserBackend(Mockito.any(RequestBuilder.class))).thenReturn(mBackend);

    Integer res = ep.getUserId(mToken, mService);
    Assert.assertEquals(new Integer(1234),res);
  }
  
  @Test
  public void testGetRequestBodyWithJsonBody() throws Exception {
    EndPoint ep = new EndPoint();
    JsonObject obj = new JsonObject();
    obj.add("testing",new JsonPrimitive("output"));
    JsonElement elem = ep.getRequestBody(new StringReader(obj + ""));
    Assert.assertTrue(elem.isJsonObject());
    Assert.assertEquals(obj,elem);
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
