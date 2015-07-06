package com.brotherlogic.recordcollection;

import java.util.Map;

import com.brotherlogic.discogs.User;
import com.brotherlogic.discogs.backend.UserBackend;
import com.brotherlogic.discogs.backend.WebUserBackend;

import com.google.gson.Gson;
import com.google.gson.JsonElement;
import com.google.gson.JsonNull;
import com.google.gson.JsonObject;
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
    
    @Override
    public void service(final ServletRequest req, final ServletResponse res)
        throws ServletException, IOException {
        
        HttpServletRequest hReq = (HttpServletRequest) req;
        HttpServletResponse hResp = (HttpServletResponse) res;

        logger.log(Level.INFO,"Converting: " + hReq.getRequestURI());
        String[] paras = hReq.getRequestURI().substring(1).split("/");

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
                DiscogsService service = (DiscogsService)((Config) req.getServletContext().getAttribute("config")).getService();
                DiscogsToken authToken = ((Map<String,DiscogsToken>)req.getServletContext().getAttribute("auth_tokens")).get(hReq.getParameter("token"));
                logger.log(Level.INFO,"Retrieved " + authToken + " from " + hReq.getParameter("token"));
                if (authToken == null) {
                    //Needs to reauth - force a redirect
                    JsonObject obj = new JsonObject();
                    obj.add("redirect",new JsonPrimitive("/index.html?reauth=true"));
                    writeResponse(hResp,obj);
                    return;
                }
                UserBackend backend = authToken.getUserBackend(service.getRequestBuilder());
                writeResponse(hResp,new Gson().toJsonTree(backend.getMe()));
                return;
            }
        }

        //Write out a json null
        writeResponse(hResp,JsonNull.INSTANCE);
    }


    private Token saveToken(String token, String verifierStr, ServletRequest req) {
        logger.log(Level.INFO,"Getting " + token + " from " + req.getServletContext().getAttribute("token_map"));
        Token prevToken = (Token) ((Map)req.getServletContext().getAttribute("token_map")).get(token);
        Verifier verifier = new Verifier(verifierStr);
        DiscogsService service = (DiscogsService)((Config) req.getServletContext().getAttribute("config")).getService();
        DiscogsToken accessToken = new DiscogsToken(service.getAccessToken(prevToken, verifier), service);

        logger.log(Level.INFO,"Received " + accessToken + " from " + prevToken + " and " + verifier);
        logger.log(Level.INFO,"Put " + accessToken.getToken() + " into auth_tokens map");
        ((Map)req.getServletContext().getAttribute("auth_tokens")).put(accessToken.getToken(),accessToken);
        return accessToken;
    }

    private String getAuthUrl(final HttpServletRequest req) {
        logger.log(Level.INFO,"Config object: " + req.getServletContext().getAttribute("config"));
        DiscogsService service = (DiscogsService)((Config) req.getServletContext().getAttribute("config")).getService();

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
