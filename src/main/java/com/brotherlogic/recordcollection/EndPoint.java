package com.brotherlogic.recordcollection;

import java.io.Reader;
import java.util.Collection;
import java.util.List;
import java.util.Map;
import java.util.TreeMap;

import com.brotherlogic.discogs.Folder;
import com.brotherlogic.discogs.User;
import com.brotherlogic.discogs.backend.CollectionBackend;
import com.brotherlogic.discogs.backend.UserBackend;
import com.brotherlogic.discogs.backend.WebCollectionBackend;
import com.brotherlogic.discogs.backend.WebUserBackend;

import com.google.gson.Gson;
import com.google.gson.JsonElement;
import com.google.gson.JsonNull;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import com.google.gson.JsonPrimitive;

import org.apache.log4j.Level;
import org.apache.log4j.Logger;

import org.scribe.oauth.OAuthService;
import org.scribe.builder.ServiceBuilder;
import org.scribe.model.OAuthRequest;
import org.scribe.model.Token;
import org.scribe.model.Verb;
import org.scribe.model.Verifier;

import java.io.IOException;
import java.io.PrintStream;

import javax.servlet.GenericServlet;
import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpServletResponse;
import javax.servlet.ServletException;
import javax.servlet.ServletRequest;
import javax.servlet.ServletResponse;

public class EndPoint extends GenericServlet {
    
  private Logger logger = Logger.getLogger(getClass());
  private String callbackURL = "http://blah";

  //Hold a mapping between tokens and user ids in memory
  private Map<String,Integer> userIDMap = new TreeMap<String,Integer>();

  protected void addUserId(DiscogsToken t, Integer idNumber) {
    userIDMap.put(t.getToken(),idNumber);
  }
  
  protected Integer getUserId(DiscogsToken t, DiscogsService service) {
    logger.log(Level.INFO,"Token = " + t);
    logger.log(Level.INFO,"USERID with " + t.getToken() + " given " + userIDMap + " and also " + t.getClass());
    if (userIDMap.containsKey(t.getToken()))
      return userIDMap.get(t.getToken());

    logger.log(Level.INFO,"Building user with " + t.getUserBackend(service.getRequestBuilder()));
    User u = t.getUserBackend(service.getRequestBuilder()).getMe();
    logger.log(Level.INFO,"Found user " + u);
    userIDMap.put(t.getToken(), u.getId());
    return u.getId();
  }
  
  @Override
  public void service(final ServletRequest req, final ServletResponse res)
    throws ServletException, IOException {
        
    HttpServletRequest hReq = (HttpServletRequest) req;
    HttpServletResponse hResp = (HttpServletResponse) res;

    logger.log(Level.INFO,"Converting: ("+hReq.getMethod()+")" + hReq.getRequestURI());
    String[] paras = hReq.getRequestURI().substring(1).split("/");
    if (hReq.getRequestURI().contains("?"))
      paras = hReq.getRequestURI().substring(1,hReq.getRequestURI().indexOf('?')).split("/");

    //Re-Auth if we need to
    DiscogsToken authToken = null;
    if (hReq.getParameter("token") != null) {
      Token tempToken = (((RcSystem)req.getServletContext().getAttribute("system")).getStorage().getToken(hReq.getParameter("token")));
      logger.log(Level.INFO,"Retrieved " + tempToken + " from " + hReq.getParameter("token"));
      if (tempToken == null) {
        //Needs to reauth - force a redirect
        JsonObject obj = new JsonObject();
        obj.add("redirect",new JsonPrimitive("/index.html?reauth=true"));
        writeResponse(hResp,obj);
        return;
      } else {
        logger.log(Level.INFO,"Building from token " + tempToken + " with " + tempToken.getToken() + "," + tempToken.getSecret() + " class = " + tempToken.getClass() + " and " + (tempToken instanceof DiscogsToken));
        logger.log(Level.INFO,"Using " + ((RcSystem)req.getServletContext().getAttribute("system")));
        logger.log(Level.INFO,"Using " + ((RcSystem)req.getServletContext().getAttribute("system")).getConfig());
        if (!(tempToken instanceof DiscogsToken))
          authToken = new DiscogsToken(tempToken,((RcSystem)req.getServletContext().getAttribute("system")).getConfig().getService());
        else
          authToken = (DiscogsToken)tempToken;
      }
    }

    logger.log(Level.INFO,"For Request " + hReq.getRequestURI() + " => " + authToken);
    
    if (paras.length > 1) {
      if (paras[1].equals("login")) {
        JsonObject obj = new JsonObject();
        obj.add("redirect",new JsonPrimitive(getAuthUrl(hReq)));
        writeResponse(hResp,obj);
        return;
      } else if (paras[1].startsWith("callback")) {
        logger.log(Level.INFO,"Request = " + hReq.getRequestURI());
        Token token = saveToken(hReq.getParameter("oauth_token"),hReq.getParameter("oauth_verifier"), req);
        JsonObject obj = new JsonObject();
        obj.add("token",new JsonPrimitive(token.getToken()));

        //Forward the browser on to /index.html?token=blah
        hResp.sendRedirect("/index.html?token=" + token.getToken());
                
        return;
      } else if (paras[1].startsWith("me")) {
        DiscogsService service = (DiscogsService)((RcSystem) req.getServletContext().getAttribute("system")).getConfig().getService();
        logger.log(Level.INFO,"Building backend " + authToken);
        UserBackend backend = authToken.getUserBackend(service.getRequestBuilder());
        logger.log(Level.INFO,"Got backend: " + backend);
        writeResponse(hResp,new Gson().toJsonTree(backend.getMe()));
        return;
      } else if (paras[1].startsWith("overview")) {
        DiscogsService service = (DiscogsService)((RcSystem) req.getServletContext().getAttribute("system")).getConfig().getService();
        CollectionBackend backend = authToken.getCollectionBackend(service.getRequestBuilder());
        Collection<Folder> folders = backend.getFolders(paras[2]);

        int colSize = 0;
        for(Folder f : folders) {
          colSize += f.getCount();
        }
                
        JsonObject response = new JsonObject();
        response.add("number_of_folders",new JsonPrimitive(folders.size()));
        response.add("collection_size",new JsonPrimitive(colSize));
        writeResponse(hResp, response);
        return;
      } else if (paras[1].startsWith("collections")) {
        RcSystem system = ((RcSystem)req.getServletContext().getAttribute("system"));
        List<FolderGroup> cols = system.getStorage().getCollections(getUserId(authToken,system.getConfig().getService()));

        writeResponse(hResp, new Gson().toJsonTree(cols));
        return;
      }
      else if (paras[1].startsWith("collection")) {
        if (hReq.getMethod().equals("PUT")) {
          JsonObject obj = getRequestBody(hReq.getReader()).getAsJsonObject();
          FolderGroup col = new FolderGroup(obj);
          RcSystem system = ((RcSystem)req.getServletContext().getAttribute("system"));
          system.getStorage().storeCollection(getUserId(authToken,system.getConfig().getService()),col);
          JsonObject response = new JsonObject();
          response.add("response",new JsonPrimitive(true));
          writeResponse(hResp,response);
          return;
        } else {
          RcSystem system = ((RcSystem)req.getServletContext().getAttribute("system"));
          logger.log(Level.INFO,"Assessing: " + authToken + " and " + system.getConfig().getService());
          logger.log(Level.INFO,"Retrieving collection: " + getUserId(authToken,system.getConfig().getService()) + " and " + hReq.getParameter("name"));
          FolderGroup col = system.getStorage().getCollection(getUserId(authToken,system.getConfig().getService()), hReq.getParameter("name"));
          logger.log(Level.INFO,"Found collection: " + col);
          writeResponse(hResp, new Gson().toJsonTree(col));
          return;
        }
      }
    }

    //Write out a json null
    writeResponse(hResp,JsonNull.INSTANCE);
  }

  /**                                                                                                                                                                              
   * @param reader                                                                                                                               
   *            The reader processing this request
   * @return The extracted body of the request                                                                                                                                     
   * @throws IOException                                                                                                                                                           
   *             If we can't process the reqesut                                                                                                                                   
   */                                                                                                                                                                              
  protected JsonElement getRequestBody(final Reader reader) throws IOException {                                                                           
    StringBuffer jb = new StringBuffer();                                                                                                                                        
    char[] buffer = new char[1024];                                                                                                                                
    int read = reader.read(buffer);                                                                                                                                         
    while (read > 0) {                                                                                                                                                  
      jb.append(new String(buffer, 0, read));                                                                                                                             
      read = reader.read(buffer);                                                                                                                                      
    }                                                                                                                                                               
    reader.close();                                                                                                                                                     
    return new JsonParser().parse(jb.toString());  
  }
  

  private Token saveToken(String token, String verifierStr, ServletRequest req) {
    logger.log(Level.INFO,"Getting " + token + " from " + req.getServletContext().getAttribute("token_map"));
    Token prevToken = (Token) ((Map)req.getServletContext().getAttribute("token_map")).get(token);
    Verifier verifier = new Verifier(verifierStr);
    DiscogsService service = (DiscogsService)((RcSystem) req.getServletContext().getAttribute("system")).getConfig().getService();
    DiscogsToken accessToken = new DiscogsToken(service.getAccessToken(prevToken, verifier), service);

    logger.log(Level.INFO,"Received " + accessToken + " from " + prevToken + " and " + verifier);
    logger.log(Level.INFO,"Put " + accessToken.getToken() + " into auth_tokens map");
    ((RcSystem)req.getServletContext().getAttribute("system")).getStorage().storeToken(accessToken);
    return accessToken;
  }

  private String getAuthUrl(final HttpServletRequest req) {
    logger.log(Level.INFO,"Servlet context: " + req.getClass());
    logger.log(Level.INFO,"Config object: " + req.getServletContext().getAttribute("config"));
    DiscogsService service = (DiscogsService)((RcSystem) req.getServletContext().getAttribute("system")).getConfig().getService();

    logger.log(Level.INFO,"Setting callback URI from : " + req.getRequestURL());
    service.setCallback(req.getRequestURL().toString().replace("login","callback"));
    Token t = service.getRequestToken();

    Map<String, Token> tokenMap = (Map) req.getServletContext().getAttribute("token_map");
    tokenMap.put(t.getToken(),t);
        
    return service.getAuthorizationUrl(t);
  }
    
  private void writeResponse (HttpServletResponse resp, JsonElement obj) throws IOException {
    logger.log(Level.INFO,"Writing " + obj + " to output stream (" + resp.getOutputStream().getClass() + ")");
    PrintStream ps = new PrintStream(resp.getOutputStream());
    ps.print(obj.toString());
    ps.close();
  }    
}
